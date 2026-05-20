package newsensationsutil

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const defaultDelay = 500 * time.Millisecond

type SiteConfig struct {
	SiteID     string
	Domain     string
	SiteBase   string
	TourPrefix string
	StudioName string
	AltDomains []string
}

type Scraper struct {
	config  SiteConfig
	client  *http.Client
	matchRe *regexp.Regexp
}

func New(cfg SiteConfig) *Scraper {
	pattern := regexp.QuoteMeta(cfg.Domain)
	for _, alt := range cfg.AltDomains {
		pattern += "|" + regexp.QuoteMeta(alt)
	}
	re := regexp.MustCompile(`^https?://(?:www\.)?(?:` + pattern + `)`)

	return &Scraper{
		config:  cfg,
		client:  httpx.NewClient(30 * time.Second),
		matchRe: re,
	}
}

func (s *Scraper) ID() string { return s.config.SiteID }

func (s *Scraper) Patterns() []string {
	tp := s.config.TourPrefix
	return []string{
		s.config.Domain,
		s.config.Domain + "/" + tp + "/categories/movies_{page}_d.html",
		s.config.Domain + "/" + tp + "/models/{name}.html",
		s.config.Domain + "/" + tp + "/categories/{category}_{page}_d.html",
	}
}

func (s *Scraper) MatchesURL(u string) bool { return s.matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

type workItem struct {
	url        string
	id         string
	title      string
	performers []string
	thumb      string
	preview    string
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	delay := opts.Delay
	if delay == 0 {
		delay = defaultDelay
	}

	workers := opts.Workers
	if workers <= 0 {
		workers = 4
	}

	work := make(chan workItem)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range work {
				scene, err := s.fetchDetail(ctx, item, studioURL, delay)
				if err != nil {
					select {
					case out <- scraper.Error(err):
					case <-ctx.Done():
						return
					}
					continue
				}
				select {
				case out <- scraper.Scene(scene):
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	s.produceListing(ctx, studioURL, opts, out, work)
	close(work)
	wg.Wait()
}

func (s *Scraper) produceListing(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult, work chan<- workItem) {
	delay := opts.Delay
	if delay == 0 {
		delay = defaultDelay
	}

	modelRe := regexp.MustCompile(`/` + regexp.QuoteMeta(s.config.TourPrefix) + `/models/`)
	if modelRe.MatchString(studioURL) {
		s.scrapeListingPage(ctx, studioURL, opts, out, work)
		return
	}

	catRe := regexp.MustCompile(`/categories/([A-Za-z0-9_-]+)_(\d+)_([dnopuz])\.html`)
	if m := catRe.FindStringSubmatch(studioURL); m != nil {
		s.paginateCategory(ctx, m[1], m[3], opts, out, work, delay)
		return
	}

	s.paginateCategory(ctx, "movies", "d", opts, out, work, delay)
}

func (s *Scraper) paginateCategory(ctx context.Context, category, sort string, opts scraper.ListOpts, out chan<- scraper.SceneResult, work chan<- workItem, delay time.Duration) {
	totalSent := false
	for page := 1; ; page++ {
		if ctx.Err() != nil {
			return
		}

		if page > 1 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}
		}

		pageURL := fmt.Sprintf("%s/%s/categories/%s_%d_%s.html", s.config.SiteBase, s.config.TourPrefix, category, page, sort)
		items, err := s.parseListingPage(ctx, pageURL)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		if len(items) == 0 {
			return
		}

		if !totalSent {
			select {
			case out <- scraper.Progress(0):
			case <-ctx.Done():
				return
			}
			totalSent = true
		}

		for _, item := range items {
			if opts.KnownIDs[item.id] {
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
				return
			}
			select {
			case work <- item:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (s *Scraper) scrapeListingPage(ctx context.Context, pageURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult, work chan<- workItem) {
	items, err := s.parseListingPage(ctx, pageURL)
	if err != nil {
		select {
		case out <- scraper.Error(err):
		case <-ctx.Done():
		}
		return
	}

	if len(items) > 0 {
		select {
		case out <- scraper.Progress(len(items)):
		case <-ctx.Done():
			return
		}
	}

	for _, item := range items {
		if opts.KnownIDs[item.id] {
			select {
			case out <- scraper.StoppedEarly():
			case <-ctx.Done():
			}
			return
		}
		select {
		case work <- item:
		case <-ctx.Done():
			return
		}
	}
}

var (
	videoThumbRe = regexp.MustCompile(`videothumb_(\d+)`)
	titleRe      = regexp.MustCompile(`<h4><a\s+href="([^"]+)">([^<]+)</a></h4>`)
	performersRe = regexp.MustCompile(`(?s)<span class="tour_update_models">(.*?)</span>`)
	performerRe  = regexp.MustCompile(`>([^<]+)</a>`)
	thumbRe      = regexp.MustCompile(`data-src="([^"]+)"`)
	previewRe    = regexp.MustCompile(`src="([^"]*\.mp4[^"]*)"`)
)

func (s *Scraper) parseListingPage(ctx context.Context, pageURL string) ([]workItem, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     pageURL,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return nil, err
	}
	html := string(body)

	blocks := SplitVideoBlocks(html)
	items := make([]workItem, 0, len(blocks))

	for _, block := range blocks {
		item := s.parseVideoBlock(block)
		if item.id == "" || item.url == "" {
			continue
		}
		items = append(items, item)
	}

	return items, nil
}

func SplitVideoBlocks(page string) []string {
	ids := videoThumbRe.FindAllStringSubmatchIndex(page, -1)
	if len(ids) == 0 {
		return nil
	}

	var blocks []string
	for i, loc := range ids {
		start := loc[0]
		var end int
		if i+1 < len(ids) {
			end = ids[i+1][0]
		} else {
			end = len(page)
		}
		blocks = append(blocks, page[start:end])
	}
	return blocks
}

func (s *Scraper) parseVideoBlock(block string) workItem {
	var item workItem

	if m := videoThumbRe.FindStringSubmatch(block); m != nil {
		item.id = m[1]
	}

	if m := titleRe.FindStringSubmatch(block); m != nil {
		item.url = s.absoluteURL(m[1])
		item.title = m[2]
	}

	if m := performersRe.FindStringSubmatch(block); m != nil {
		matches := performerRe.FindAllStringSubmatch(m[1], -1)
		for _, pm := range matches {
			item.performers = append(item.performers, strings.TrimSpace(pm[1]))
		}
	}

	if m := thumbRe.FindStringSubmatch(block); m != nil {
		item.thumb = m[1]
	}

	if m := previewRe.FindStringSubmatch(block); m != nil {
		item.preview = strings.TrimSpace(m[1])
	}

	return item
}

var (
	detailDateRe     = regexp.MustCompile(`(?si)class="sceneDateP"><span>(\d{2}/\d{2}/\d{4}),</span>.*?(\d+)(?:\s|&nbsp;)*min`)
	detailDescRe     = regexp.MustCompile(`(?s)Description:</span>\s*(?:<h2[^>]*>)?\s*(.*?)\s*(?:</h2>|</p>)`)
	detailKeywordsRe = regexp.MustCompile(`<meta\s+name="keywords"\s+content="([^"]*)"`)
	detailIDRe       = regexp.MustCompile(`data-id="(\d+)"`)
)

func (s *Scraper) fetchDetail(ctx context.Context, item workItem, studioURL string, delay time.Duration) (models.Scene, error) {
	select {
	case <-time.After(delay):
	case <-ctx.Done():
		return models.Scene{}, ctx.Err()
	}

	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     item.url,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return models.Scene{}, fmt.Errorf("detail %s: %w", item.url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return models.Scene{}, fmt.Errorf("reading detail %s: %w", item.url, err)
	}
	page := string(body)

	var date time.Time
	var duration int
	if m := detailDateRe.FindStringSubmatch(page); m != nil {
		date, _ = time.Parse("01/02/2006", m[1])
		duration, _ = strconv.Atoi(m[2])
		duration *= 60
	}

	var description string
	if m := detailDescRe.FindStringSubmatch(page); m != nil && m[1] != "" {
		description = strings.TrimSpace(m[1])
	}

	tags, series := s.extractTagsAndSeries(page)

	if item.id == "" {
		if m := detailIDRe.FindStringSubmatch(page); m != nil {
			item.id = m[1]
		}
	}

	now := time.Now().UTC()
	return models.Scene{
		ID:          item.id,
		SiteID:      s.config.SiteID,
		StudioURL:   studioURL,
		Title:       item.title,
		URL:         item.url,
		Date:        date.UTC(),
		Description: description,
		Thumbnail:   item.thumb,
		Preview:     item.preview,
		Performers:  item.performers,
		Studio:      s.config.StudioName,
		Tags:        tags,
		Series:      series,
		Duration:    duration,
		ScrapedAt:   now,
	}, nil
}

func (s *Scraper) extractTagsAndSeries(page string) ([]string, string) {
	m := detailKeywordsRe.FindStringSubmatch(page)
	if m == nil {
		return nil, ""
	}

	skip := map[string]bool{
		"4K": true, "Updates": true, "Movies": true, "Photos": true,
		"Interactive Toys": true,
	}
	skip[s.config.StudioName] = true
	skip[s.config.Domain] = true
	domainUpper := strings.ToUpper(s.config.Domain[:1]) + s.config.Domain[1:]
	skip[domainUpper] = true
	if s.config.Domain != "newsensations.com" {
		skip["New Sensations"] = true
		skip["NewSensations.com"] = true
	}

	var tags []string
	var series string
	parts := strings.Split(m[1], ",")
	for _, p := range parts {
		tag := strings.TrimSpace(p)
		if tag == "" {
			continue
		}
		if strings.HasSuffix(tag, ".com") {
			continue
		}
		if skip[tag] {
			continue
		}
		tags = append(tags, tag)
	}

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.Contains(p, "#") {
			series = strings.TrimSpace(strings.Split(p, "#")[0])
			break
		}
	}

	return tags, series
}

func (s *Scraper) absoluteURL(href string) string {
	if strings.HasPrefix(href, "http") {
		return href
	}
	return s.config.SiteBase + "/" + strings.TrimPrefix(href, "/")
}
