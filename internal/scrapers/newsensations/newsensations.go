package newsensations

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

const (
	siteBase     = "https://www.newsensations.com"
	tourBase     = siteBase + "/tour_ns"
	siteID       = "newsensations"
	studioName   = "New Sensations"
	defaultDelay = 500 * time.Millisecond
)

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?newsensations\.com`)

type Scraper struct {
	client *http.Client
}

func New() *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second)}
}

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		"newsensations.com",
		"newsensations.com/tour_ns/categories/movies_{page}_d.html",
		"newsensations.com/tour_ns/models/{name}.html",
		"newsensations.com/tour_ns/categories/{category}_{page}_d.html",
	}
}

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

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

var (
	modelPageRe = regexp.MustCompile(`/models/([A-Za-z0-9_-]+)\.html`)
	catPageRe   = regexp.MustCompile(`/categories/([A-Za-z0-9_-]+)_(\d+)_([dnopuz])\.html`)
)

func (s *Scraper) produceListing(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult, work chan<- workItem) {
	delay := opts.Delay
	if delay == 0 {
		delay = defaultDelay
	}

	if modelPageRe.MatchString(studioURL) {
		s.scrapeListingPage(ctx, studioURL, opts, out, work)
		return
	}

	if m := catPageRe.FindStringSubmatch(studioURL); m != nil {
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

		pageURL := fmt.Sprintf("%s/categories/%s_%d_%s.html", tourBase, category, page, sort)
		items, nextExists, err := s.parseListingPage(ctx, pageURL)
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

		if !totalSent && nextExists {
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

		if !nextExists {
			return
		}
	}
}

func (s *Scraper) scrapeListingPage(ctx context.Context, pageURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult, work chan<- workItem) {
	items, _, err := s.parseListingPage(ctx, pageURL)
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
	nextPageRe   = regexp.MustCompile(`_(\d+)_[dnopuz]\.html"[^>]*>\s*(?:next|&raquo;|›)`)
)

func (s *Scraper) parseListingPage(ctx context.Context, pageURL string) ([]workItem, bool, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     pageURL,
		Headers: map[string]string{"User-Agent": httpx.UserAgentFirefox},
	})
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return nil, false, err
	}
	html := string(body)

	blocks := splitVideoBlocks(html)
	items := make([]workItem, 0, len(blocks))

	for _, block := range blocks {
		item := parseVideoBlock(block)
		if item.id == "" || item.url == "" {
			continue
		}
		items = append(items, item)
	}

	hasNext := nextPageRe.MatchString(html)

	return items, hasNext, nil
}

func splitVideoBlocks(page string) []string {
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

func parseVideoBlock(block string) workItem {
	var item workItem

	if m := videoThumbRe.FindStringSubmatch(block); m != nil {
		item.id = m[1]
	}

	if m := titleRe.FindStringSubmatch(block); m != nil {
		item.url = absoluteURL(m[1])
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
	detailDateRe     = regexp.MustCompile(`(?s)class="sceneDateP"><span>(\d{2}/\d{2}/\d{4}),</span>.*?(\d+)(?:\s|&nbsp;)*min`)
	detailDescRe     = regexp.MustCompile(`(?s)<div class="sceneRight">.*?<h1>[^<]*</h1>\s*(?:<h2[^>]*>(.*?)</h2>)?`)
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
		Headers: map[string]string{"User-Agent": httpx.UserAgentFirefox},
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

	var tags []string
	var series string
	if m := detailKeywordsRe.FindStringSubmatch(page); m != nil {
		parts := strings.Split(m[1], ",")
		for _, p := range parts {
			tag := strings.TrimSpace(p)
			if tag != "" && tag != studioName && tag != "NewSensations.com" &&
				tag != "Updates" && tag != "Movies" && tag != "Photos" {
				tags = append(tags, tag)
			}
		}
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if strings.Contains(p, "#") {
				series = strings.TrimSpace(strings.Split(p, "#")[0])
				break
			}
		}
	}

	if item.id == "" {
		if m := detailIDRe.FindStringSubmatch(page); m != nil {
			item.id = m[1]
		}
	}

	now := time.Now().UTC()
	return models.Scene{
		ID:          item.id,
		SiteID:      siteID,
		StudioURL:   studioURL,
		Title:       item.title,
		URL:         item.url,
		Date:        date.UTC(),
		Description: description,
		Thumbnail:   item.thumb,
		Preview:     item.preview,
		Performers:  item.performers,
		Studio:      studioName,
		Tags:        tags,
		Series:      series,
		Duration:    duration,
		ScrapedAt:   now,
	}, nil
}

func absoluteURL(href string) string {
	if strings.HasPrefix(href, "http") {
		return href
	}
	return siteBase + "/" + strings.TrimPrefix(href, "/")
}
