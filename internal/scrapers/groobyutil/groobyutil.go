package groobyutil

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/parseutil"
	"github.com/Wasylq/FSS/scraper"
)

type SiteConfig struct {
	SiteID     string
	Domain     string
	StudioName string
	TourPrefix string // "/tour" or "" for sites without /tour/
	AltDomains []string
}

type Scraper struct {
	cfg    SiteConfig
	client *http.Client
	base   string
}

func New(cfg SiteConfig) *Scraper {
	return &Scraper{
		cfg:    cfg,
		client: httpx.NewClient(30 * time.Second),
		base:   "https://www." + cfg.Domain,
	}
}

func (s *Scraper) ID() string { return s.cfg.SiteID }

func (s *Scraper) Patterns() []string {
	d := s.cfg.Domain
	prefix := s.cfg.TourPrefix
	return []string{
		d + prefix + "/categories/movies.html",
		d + prefix + "/models/{slug}.html",
	}
}

func (s *Scraper) MatchesURL(u string) bool {
	domains := append([]string{s.cfg.Domain}, s.cfg.AltDomains...)
	for _, d := range domains {
		if strings.Contains(u, "://"+d) || strings.Contains(u, "://www."+d) {
			return true
		}
	}
	return false
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	cardRe      = regexp.MustCompile(`<div class="sexyvideo"`)
	sceneIDRe   = regexp.MustCompile(`id="set-target-(\d+)"`)
	titleRe     = regexp.MustCompile(`(?s)<h4>\s*<a[^>]+title="([^"]+)"`)
	sceneURLRe  = regexp.MustCompile(`(?s)<h4>\s*<a\s+href="([^"]+)"`)
	thumbRe     = regexp.MustCompile(`<img[^>]*class="[^"]*mainThumb[^"]*"[^>]*src="([^"]+)"`)
	durationRe  = regexp.MustCompile(`<i class='fas fa-video'></i>\s*<div[^>]*>(\d+:\d{2}(?::\d{2})?)`)
	performerRe = regexp.MustCompile(`<div class="modelname">\s*<a[^>]+><span[^>]*>([^<]+)</span></a>`)
	descRe      = regexp.MustCompile(`<p class="photodesc">([^<]+)</p>`)
	dateRe      = regexp.MustCompile(`<i class='far fa-calendar'[^>]*></i>\s*(\d{1,2}(?:st|nd|rd|th)\s+\w+\s+\d{4})`)
	maxPageRe   = regexp.MustCompile(`movies_(\d+)_d\.html`)

	modelSlugRe = regexp.MustCompile(`/models/([^_/.]+?)(?:\.html)?$`)
)

type sceneItem struct {
	id          string
	title       string
	url         string
	thumb       string
	date        time.Time
	duration    int
	performers  []string
	description string
}

func parseListingPage(body []byte) []sceneItem {
	page := string(body)
	starts := cardRe.FindAllStringIndex(page, -1)
	items := make([]sceneItem, 0, len(starts))

	for i, loc := range starts {
		start := loc[0]
		end := len(page)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		block := page[start:end]

		var item sceneItem

		if m := sceneIDRe.FindStringSubmatch(block); m != nil {
			item.id = m[1]
		}
		if item.id == "" {
			continue
		}

		if m := titleRe.FindStringSubmatch(block); m != nil {
			item.title = strings.TrimSpace(html.UnescapeString(m[1]))
		}

		if m := sceneURLRe.FindStringSubmatch(block); m != nil {
			item.url = strings.TrimSpace(m[1])
		}

		if m := thumbRe.FindStringSubmatch(block); m != nil {
			item.thumb = m[1]
		}

		if m := durationRe.FindStringSubmatch(block); m != nil {
			item.duration = parseutil.ParseDurationColon(m[1])
		}

		for _, m := range performerRe.FindAllStringSubmatch(block, -1) {
			name := strings.TrimSpace(html.UnescapeString(m[1]))
			if name != "" {
				item.performers = append(item.performers, name)
			}
		}

		if m := descRe.FindStringSubmatch(block); m != nil {
			item.description = strings.TrimSpace(html.UnescapeString(m[1]))
		}

		if m := dateRe.FindStringSubmatch(block); m != nil {
			item.date = parseGroobyDate(m[1])
		}

		items = append(items, item)
	}
	return items
}

// parseGroobyDate parses "8th May 2026" → time.Time.
func parseGroobyDate(s string) time.Time {
	cleaned := parseutil.StripOrdinalSuffix(s)
	t, err := time.Parse("2 January 2006", cleaned)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

func estimateTotal(body []byte, perPage int) int {
	max := 1
	for _, m := range maxPageRe.FindAllSubmatch(body, -1) {
		n, _ := strconv.Atoi(string(m[1]))
		if n > max {
			max = n
		}
	}
	return max * perPage
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	base := s.base

	if modelSlugRe.MatchString(studioURL) {
		scraper.Debugf(1, "%s: detected model page", s.cfg.SiteID)
		s.scrapeModelPage(ctx, studioURL, opts, out, now, base)
		return
	}

	s.scrapeListingPages(ctx, opts, out, now, base)
}

func (s *Scraper) scrapeModelPage(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult, now time.Time, base string) {
	pageURL := studioURL
	if !strings.HasPrefix(pageURL, "http") {
		pageURL = base + pageURL
	}

	body, err := s.fetchPage(ctx, pageURL)
	if err != nil {
		select {
		case out <- scraper.Error(err):
		case <-ctx.Done():
		}
		return
	}

	scenes := parseListingPage(body)
	if len(scenes) == 0 {
		return
	}
	scraper.Debugf(1, "%s: found %d scenes on model page", s.cfg.SiteID, len(scenes))

	select {
	case out <- scraper.Progress(len(scenes)):
	case <-ctx.Done():
		return
	}

	for _, item := range scenes {
		if opts.KnownIDs[item.id] {
			scraper.Debugf(1, "%s: hit known ID %s, stopping early", s.cfg.SiteID, item.id)
			select {
			case out <- scraper.StoppedEarly():
			case <-ctx.Done():
			}
			return
		}
		select {
		case out <- scraper.Scene(item.toScene(s.cfg.SiteID, s.cfg.StudioName, base, now)):
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scraper) scrapeListingPages(ctx context.Context, opts scraper.ListOpts, out chan<- scraper.SceneResult, now time.Time, base string) {
	for page := 1; ; page++ {
		if ctx.Err() != nil {
			return
		}
		if page > 1 && opts.Delay > 0 {
			select {
			case <-time.After(opts.Delay):
			case <-ctx.Done():
				return
			}
		}

		scraper.Debugf(1, "%s: fetching page %d", s.cfg.SiteID, page)
		pageURL := fmt.Sprintf("%s%s/categories/movies_%d_d.html", base, s.cfg.TourPrefix, page)

		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		scenes := parseListingPage(body)
		if len(scenes) == 0 {
			return
		}

		if page == 1 {
			total := estimateTotal(body, len(scenes))
			scraper.Debugf(1, "%s: %d total scenes (estimated)", s.cfg.SiteID, total)
			if total > 0 {
				select {
				case out <- scraper.Progress(total):
				case <-ctx.Done():
					return
				}
			}
		}

		for _, item := range scenes {
			if opts.KnownIDs[item.id] {
				scraper.Debugf(1, "%s: hit known ID %s, stopping early", s.cfg.SiteID, item.id)
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
				return
			}
			select {
			case out <- scraper.Scene(item.toScene(s.cfg.SiteID, s.cfg.StudioName, base, now)):
			case <-ctx.Done():
				return
			}
		}
	}
}

func (item sceneItem) toScene(siteID, studio, base string, now time.Time) models.Scene {
	url := item.url
	if strings.HasPrefix(url, "/") {
		url = base + url
	}
	thumb := item.thumb
	if strings.HasPrefix(thumb, "/") {
		thumb = base + thumb
	}
	return models.Scene{
		ID:          item.id,
		SiteID:      siteID,
		StudioURL:   base,
		Title:       item.title,
		URL:         url,
		Thumbnail:   thumb,
		Date:        item.date,
		Duration:    item.duration,
		Performers:  item.performers,
		Description: item.description,
		Studio:      studio,
		ScrapedAt:   now,
	}
}

func (s *Scraper) fetchPage(ctx context.Context, url string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     url,
		Headers: httpx.BrowserHeaders(httpx.UserAgentChrome),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}
