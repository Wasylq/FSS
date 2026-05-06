package cherrypimpsutil

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

type SiteConfig struct {
	ID       string
	SiteBase string
	Studio   string
	Patterns []string
	MatchRe  *regexp.Regexp
}

type Scraper struct {
	cfg    SiteConfig
	client *http.Client
}

func New(cfg SiteConfig) *Scraper {
	return &Scraper{
		cfg:    cfg,
		client: httpx.NewClient(30 * time.Second),
	}
}

func (s *Scraper) ID() string         { return s.cfg.ID }
func (s *Scraper) Patterns() []string { return s.cfg.Patterns }
func (s *Scraper) MatchesURL(u string) bool {
	return s.cfg.MatchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	cardStartRe = regexp.MustCompile(`<div class="item-update[^"]*item-video[^"]*">`)
	sceneIDRe   = regexp.MustCompile(`/trailers/(\d+)-`)
	titleRe     = regexp.MustCompile(`(?s)<div class="item-title">\s*<a[^>]+title="([^"]+)"`)
	sceneURLRe  = regexp.MustCompile(`(?s)<div class="item-title">\s*<a\s+href="([^"]+)"`)
	thumbRe     = regexp.MustCompile(`<img[^>]+src="([^"]+)"[^>]+class="video_placeholder"`)
	dateRe      = regexp.MustCompile(`([A-Z][a-z]+ \d{1,2}, \d{4})`)
	durationRe  = regexp.MustCompile(`(\d+:\d{2})`)
	performerRe = regexp.MustCompile(`<a[^>]+href="[^"]*models/[^"]*">([^<]+)</a>`)
	maxPageRe   = regexp.MustCompile(`_(\d+)_d\.html`)

	seriesSlugRe   = regexp.MustCompile(`/series/([^_/.]+)`)
	categorySlugRe = regexp.MustCompile(`/categories/([^_/.]+)`)
	modelSlugRe    = regexp.MustCompile(`/models/([^_/.]+?)(?:\.html)?$`)
)

const cardEnd = "<!--//item-update-->"

type sceneItem struct {
	id         string
	title      string
	url        string
	thumb      string
	date       time.Time
	duration   int
	performers []string
}

func parseListingPage(body []byte) []sceneItem {
	page := string(body)
	starts := cardStartRe.FindAllStringIndex(page, -1)
	items := make([]sceneItem, 0, len(starts))

	for _, loc := range starts {
		rest := page[loc[0]:]
		endIdx := strings.Index(rest, cardEnd)
		if endIdx < 0 {
			continue
		}
		block := rest[:endIdx]

		var item sceneItem

		if sm := sceneIDRe.FindStringSubmatch(block); sm != nil {
			item.id = sm[1]
		}
		if item.id == "" {
			continue
		}

		if sm := titleRe.FindStringSubmatch(block); sm != nil {
			item.title = strings.TrimSpace(sm[1])
		}

		if sm := sceneURLRe.FindStringSubmatch(block); sm != nil {
			item.url = sm[1]
		}

		if sm := thumbRe.FindStringSubmatch(block); sm != nil {
			item.thumb = sm[1]
		}

		if idx := strings.Index(block, `class="item-date"`); idx >= 0 {
			dateSection := block[idx:]
			if end := strings.Index(dateSection, "</div>"); end > 0 {
				dateSection = dateSection[:end]
				if sm := durationRe.FindStringSubmatch(dateSection); sm != nil {
					item.duration = parseDuration(sm[1])
				}
				if sm := dateRe.FindStringSubmatch(dateSection); sm != nil {
					if t, err := time.Parse("January 2, 2006", sm[1]); err == nil {
						item.date = t.UTC()
					}
				}
			}
		}

		if idx := strings.Index(block, `class="item-models"`); idx >= 0 {
			perfSection := block[idx:]
			if end := strings.Index(perfSection, "</div>"); end > 0 {
				perfSection = perfSection[:end]
				for _, pm := range performerRe.FindAllStringSubmatch(perfSection, -1) {
					name := strings.TrimSpace(pm[1])
					if name != "" {
						item.performers = append(item.performers, name)
					}
				}
			}
		}

		items = append(items, item)
	}
	return items
}

func parseDuration(s string) int {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0
	}
	mins, _ := strconv.Atoi(parts[0])
	secs, _ := strconv.Atoi(parts[1])
	return mins*60 + secs
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

type listingMode int

const (
	modeFullCatalog listingMode = iota
	modeSeries
	modeCategory
	modeModel
)

type listingConfig struct {
	mode listingMode
	slug string
}

func parseStudioURL(studioURL string) listingConfig {
	if m := modelSlugRe.FindStringSubmatch(studioURL); m != nil {
		return listingConfig{mode: modeModel, slug: m[1]}
	}
	if m := seriesSlugRe.FindStringSubmatch(studioURL); m != nil {
		return listingConfig{mode: modeSeries, slug: m[1]}
	}
	if m := categorySlugRe.FindStringSubmatch(studioURL); m != nil {
		slug := m[1]
		if slug != "movies" {
			return listingConfig{mode: modeCategory, slug: slug}
		}
	}
	return listingConfig{mode: modeFullCatalog}
}

func (lc listingConfig) pageURL(base string, page int) string {
	switch lc.mode {
	case modeSeries:
		return fmt.Sprintf("%s/series/%s_%d_d.html", base, lc.slug, page)
	case modeCategory:
		return fmt.Sprintf("%s/categories/%s_%d_d.html", base, lc.slug, page)
	default:
		return fmt.Sprintf("%s/categories/movies_%d_d.html", base, page)
	}
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	lc := parseStudioURL(studioURL)
	now := time.Now().UTC()

	if lc.mode == modeModel {
		s.scrapeModelPage(ctx, studioURL, opts, out, now)
		return
	}

	s.scrapeListingPages(ctx, lc, opts, out, now)
}

func (s *Scraper) scrapeModelPage(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult, now time.Time) {
	pageURL := studioURL
	if !strings.HasPrefix(pageURL, "http") {
		pageURL = s.cfg.SiteBase + pageURL
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

	select {
	case out <- scraper.Progress(len(scenes)):
	case <-ctx.Done():
		return
	}

	for _, item := range scenes {
		if opts.KnownIDs[item.id] {
			select {
			case out <- scraper.StoppedEarly():
			case <-ctx.Done():
			}
			return
		}
		select {
		case out <- scraper.Scene(item.toScene(s.cfg.ID, s.cfg.SiteBase, s.cfg.Studio, now)):
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scraper) scrapeListingPages(ctx context.Context, lc listingConfig, opts scraper.ListOpts, out chan<- scraper.SceneResult, now time.Time) {
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

		pageURL := lc.pageURL(s.cfg.SiteBase, page)

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
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
				return
			}
			select {
			case out <- scraper.Scene(item.toScene(s.cfg.ID, s.cfg.SiteBase, s.cfg.Studio, now)):
			case <-ctx.Done():
				return
			}
		}
	}
}

func (item sceneItem) toScene(siteID, siteBase, studio string, now time.Time) models.Scene {
	url := item.url
	if strings.HasPrefix(url, "/") {
		url = siteBase + url
	}
	return models.Scene{
		ID:         item.id,
		SiteID:     siteID,
		StudioURL:  siteBase,
		Title:      item.title,
		URL:        url,
		Thumbnail:  item.thumb,
		Date:       item.date,
		Duration:   item.duration,
		Performers: item.performers,
		Studio:     studio,
		ScrapedAt:  now,
	}
}

func (s *Scraper) fetchPage(ctx context.Context, url string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: url,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentChrome,
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}
