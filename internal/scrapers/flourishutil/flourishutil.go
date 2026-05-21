package flourishutil

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
		base:   "https://tour." + cfg.Domain,
	}
}

func (s *Scraper) ID() string { return s.cfg.SiteID }

func (s *Scraper) Patterns() []string {
	d := s.cfg.Domain
	return []string{
		"tour." + d + "/categories/movies.html",
		"tour." + d + "/models/{slug}.html",
	}
}

func (s *Scraper) MatchesURL(u string) bool {
	return strings.Contains(u, "://tour."+s.cfg.Domain) ||
		strings.Contains(u, "://www."+s.cfg.Domain) ||
		strings.Contains(u, "://"+s.cfg.Domain)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

const perPage = 12

var (
	cardRe     = regexp.MustCompile(`<div class="item item-video">`)
	sceneIDRe  = regexp.MustCompile(`id="set-target-(\d+)"`)
	titleRe    = regexp.MustCompile(`<a[^>]+href="[^"]*trailers/[^"]*\.html"[^>]*title="([^"]+)"`)
	sceneURLRe = regexp.MustCompile(`<a[^>]+href="([^"]*trailers/[^"]*\.html)"`)
	thumbRe    = regexp.MustCompile(`class="mainThumb[^"]*"[^>]*src="([^"]+)"`)
	durationRe = regexp.MustCompile(`(\d+:\d{2})`)
	dateRe     = regexp.MustCompile(`(\d{4}-\d{2}-\d{2})`)
	perfRe     = regexp.MustCompile(`<a\s+href="[^"]*models/[^"]+\.html">([^<]+)</a>`)
	maxPageRe  = regexp.MustCompile(`movies_(\d+)_d\.html`)

	modelSlugRe = regexp.MustCompile(`/models/[^/]+\.html$`)

	detailDescRe = regexp.MustCompile(`(?s)<div class="description">\s*<h3>[^<]*</h3>\s*<p>(.*?)</p>`)
	detailTagsRe = regexp.MustCompile(`(?s)<ul class="tags">(.*?)</ul>`)
	detailTagRe  = regexp.MustCompile(`<a[^>]*>([^<]+)</a>`)

	brTagRe    = regexp.MustCompile(`<br\s*/?\s*>`)
	stripTagRe = regexp.MustCompile(`<[^>]+>`)
)

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

		if m := dateRe.FindStringSubmatch(block); m != nil {
			if t, err := time.Parse("2006-01-02", m[1]); err == nil {
				item.date = t.UTC()
			}
		}

		for _, m := range perfRe.FindAllStringSubmatch(block, -1) {
			name := strings.TrimSpace(html.UnescapeString(m[1]))
			if name != "" {
				item.performers = append(item.performers, name)
			}
		}

		items = append(items, item)
	}
	return items
}

type detailData struct {
	description string
	tags        []string
}

func parseDetailPage(body []byte) detailData {
	var d detailData

	if m := detailDescRe.FindSubmatch(body); m != nil {
		raw := strings.TrimSpace(string(m[1]))
		raw = brTagRe.ReplaceAllString(raw, "\n")
		raw = stripTagRe.ReplaceAllString(raw, "")
		d.description = strings.TrimSpace(html.UnescapeString(raw))
	}

	if m := detailTagsRe.FindSubmatch(body); m != nil {
		for _, tm := range detailTagRe.FindAllSubmatch(m[1], -1) {
			tag := strings.TrimSpace(string(tm[1]))
			if tag != "" {
				d.tags = append(d.tags, tag)
			}
		}
	}

	return d
}

func estimateTotal(body []byte) int {
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

	if modelSlugRe.MatchString(studioURL) {
		s.scrapeModelPage(ctx, studioURL, opts, out, now)
		return
	}

	s.scrapeListingPages(ctx, opts, out, now)
}

func (s *Scraper) scrapeModelPage(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult, now time.Time) {
	pageURL := studioURL
	if !strings.HasPrefix(pageURL, "http") {
		pageURL = s.base + pageURL
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

		scene := s.buildScene(ctx, item, now)
		select {
		case out <- scraper.Scene(scene):
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scraper) scrapeListingPages(ctx context.Context, opts scraper.ListOpts, out chan<- scraper.SceneResult, now time.Time) {
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

		pageURL := fmt.Sprintf("%s/categories/movies_%d_d.html", s.base, page)

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
			total := estimateTotal(body)
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

			scene := s.buildScene(ctx, item, now)
			select {
			case out <- scraper.Scene(scene):
			case <-ctx.Done():
				return
			}
		}

		if len(scenes) < perPage {
			return
		}
	}
}

func (s *Scraper) buildScene(ctx context.Context, item sceneItem, now time.Time) models.Scene {
	url := item.url
	if strings.HasPrefix(url, "/") {
		url = s.base + url
	}
	thumb := item.thumb
	if strings.HasPrefix(thumb, "/") {
		thumb = s.base + thumb
	}

	var desc string
	var tags []string
	if item.url != "" {
		detailURL := item.url
		if strings.HasPrefix(detailURL, "/") {
			detailURL = s.base + detailURL
		}
		if body, err := s.fetchPage(ctx, detailURL); err == nil {
			detail := parseDetailPage(body)
			desc = detail.description
			tags = detail.tags
		}
	}

	return models.Scene{
		ID:          item.id,
		SiteID:      s.cfg.SiteID,
		StudioURL:   s.base,
		Title:       item.title,
		URL:         url,
		Thumbnail:   thumb,
		Date:        item.date,
		Duration:    item.duration,
		Performers:  item.performers,
		Description: desc,
		Tags:        tags,
		Studio:      s.cfg.StudioName,
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
