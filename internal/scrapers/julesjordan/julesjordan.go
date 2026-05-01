package julesjordan

import (
	"context"
	"fmt"
	"html"
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
	defaultBase  = "https://www.julesjordan.com/trial"
	siteID       = "julesjordan"
	defaultDelay = 500 * time.Millisecond
)

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?julesjordan\.com`)

type Scraper struct {
	client *http.Client
	base   string
}

func New() *Scraper {
	return &Scraper{
		client: httpx.NewClient(30 * time.Second),
		base:   defaultBase,
	}
}

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		"julesjordan.com/trial/",
		"julesjordan.com/trial/categories/movies.html",
		"julesjordan.com/trial/models/{slug}.html",
		"julesjordan.com/trial/dvds/dvds.html",
	}
}

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// --- runner ---

type workItem struct {
	url        string
	slug       string
	title      string
	performers []string
	date       time.Time
	thumb      string
	series     string
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

	go func() {
		defer close(work)
		switch {
		case strings.Contains(studioURL, "/models/"):
			s.enqueueModelPage(ctx, studioURL, opts, out, work)
		case strings.Contains(studioURL, "/dvds/"):
			s.enqueueDVDPages(ctx, delay, opts, out, work)
		default:
			s.enqueueListingPages(ctx, delay, opts, out, work)
		}
	}()

	wg.Wait()
}

// --- listing pages ---

func (s *Scraper) enqueueListingPages(ctx context.Context, delay time.Duration, opts scraper.ListOpts, out chan<- scraper.SceneResult, work chan<- workItem) {
	for page := 1; ; page++ {
		if ctx.Err() != nil {
			return
		}
		if page > 1 && delay > 0 {
			select {
			case <-time.After(delay):
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

		items := parseListingCards(body, s.base)
		if len(items) == 0 {
			return
		}

		if page == 1 {
			maxPage := extractMaxListPage(body)
			if maxPage > 0 {
				select {
				case out <- scraper.Progress(maxPage * len(items)):
				case <-ctx.Done():
					return
				}
			}
		}

		for _, item := range items {
			if opts.KnownIDs[item.slug] {
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

// --- model page ---

func (s *Scraper) enqueueModelPage(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult, work chan<- workItem) {
	body, err := s.fetchPage(ctx, studioURL)
	if err != nil {
		select {
		case out <- scraper.Error(err):
		case <-ctx.Done():
		}
		return
	}

	items := parseListingCards(body, s.base)
	if len(items) == 0 {
		return
	}

	select {
	case out <- scraper.Progress(len(items)):
	case <-ctx.Done():
		return
	}

	for _, item := range items {
		if opts.KnownIDs[item.slug] {
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

// --- DVD pages ---

type dvdEntry struct {
	url  string
	name string
}

func (s *Scraper) enqueueDVDPages(ctx context.Context, delay time.Duration, opts scraper.ListOpts, out chan<- scraper.SceneResult, work chan<- workItem) {
	seen := map[string]bool{}

	for page := 1; ; page++ {
		if ctx.Err() != nil {
			return
		}
		if page > 1 && delay > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}
		}

		var pageURL string
		if page == 1 {
			pageURL = s.base + "/dvds/dvds.html"
		} else {
			pageURL = fmt.Sprintf("%s/dvds/dvds_page_%d.html?s=d", s.base, page)
		}

		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("dvd page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		dvds := parseDVDListingPage(body)
		if len(dvds) == 0 {
			return
		}

		for _, dvd := range dvds {
			if ctx.Err() != nil {
				return
			}
			if delay > 0 {
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return
				}
			}

			dvdURL := dvd.url
			if strings.HasPrefix(dvdURL, "/") {
				dvdURL = s.base + dvdURL
			}

			dvdBody, err := s.fetchPage(ctx, dvdURL)
			if err != nil {
				select {
				case out <- scraper.Error(fmt.Errorf("dvd %s: %w", dvd.name, err)):
				case <-ctx.Done():
				}
				continue
			}

			sceneURLs := extractDVDSceneURLs(dvdBody)
			for _, rawURL := range sceneURLs {
				sceneURL := rawURL
				if strings.HasPrefix(sceneURL, "/") {
					sceneURL = s.base + sceneURL
				}
				sm := slugRe.FindStringSubmatch(sceneURL)
				if sm == nil {
					continue
				}
				slug := sm[1]

				if seen[slug] {
					continue
				}
				seen[slug] = true

				if opts.KnownIDs[slug] {
					select {
					case out <- scraper.StoppedEarly():
					case <-ctx.Done():
					}
					return
				}

				select {
				case work <- workItem{url: sceneURL, slug: slug, series: dvd.name}:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

// --- parsing: listing cards ---

var (
	cardRe        = regexp.MustCompile(`class="jj-content-card"`)
	sceneURLRe    = regexp.MustCompile(`href="([^"]*scenes/[^"]*_vids\.html)"`)
	slugRe        = regexp.MustCompile(`/scenes/([^/]+?)_vids\.html`)
	cardThumbRe   = regexp.MustCompile(`class="jj-thumb-img[^"]*"[^>]*src="([^"]+)"`)
	cardTitleRe   = regexp.MustCompile(`class="jj-card-title"[^>]*>([^<]+)<`)
	performerRe   = regexp.MustCompile(`href="[^"]*models/[^"]*">([^<]+)</a>`)
	cardDateRe    = regexp.MustCompile(`class="jj-card-date"[^>]*>Released:\s*([^<]+)<`)
	listPageNumRe = regexp.MustCompile(`movies_(\d+)_d\.html`)
)

func extractMaxListPage(body []byte) int {
	matches := listPageNumRe.FindAllSubmatch(body, -1)
	max := 0
	for _, m := range matches {
		n, _ := strconv.Atoi(string(m[1]))
		if n > max {
			max = n
		}
	}
	return max
}

func parseListingCards(body []byte, base string) []workItem {
	page := string(body)
	locs := cardRe.FindAllStringIndex(page, -1)
	items := make([]workItem, 0, len(locs))

	for i, loc := range locs {
		start := loc[0]
		end := len(page)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		block := page[start:end]

		urlMatch := sceneURLRe.FindStringSubmatch(block)
		if urlMatch == nil {
			continue
		}
		sceneURL := urlMatch[1]
		if strings.HasPrefix(sceneURL, "/") {
			sceneURL = base + sceneURL
		}

		sm := slugRe.FindStringSubmatch(sceneURL)
		if sm == nil {
			continue
		}

		item := workItem{url: sceneURL, slug: sm[1]}

		if m := cardTitleRe.FindStringSubmatch(block); m != nil {
			item.title = strings.TrimSpace(html.UnescapeString(m[1]))
		}

		for _, m := range performerRe.FindAllStringSubmatch(block, -1) {
			name := strings.TrimSpace(html.UnescapeString(m[1]))
			if name != "" {
				item.performers = append(item.performers, name)
			}
		}

		if m := cardDateRe.FindStringSubmatch(block); m != nil {
			raw := strings.TrimSpace(m[1])
			if t, err := time.Parse("January 2, 2006", raw); err == nil {
				item.date = t.UTC()
			}
		}

		if m := cardThumbRe.FindStringSubmatch(block); m != nil {
			item.thumb = m[1]
		}

		items = append(items, item)
	}
	return items
}

// --- parsing: detail page ---

var (
	detailTitleRe   = regexp.MustCompile(`class="scene-title"[^>]*>([^<]+)<`)
	detailDescRe    = regexp.MustCompile(`(?s)class="scene-desc"[^>]*>(.*?)</div>`)
	catTagRe        = regexp.MustCompile(`class="cat-tag"[^>]*>([^<]+)</a>`)
	starringBlockRe = regexp.MustCompile(`(?s)"lbl">Starring</div>\s*<div class="val"[^>]*>(.*?)</div>`)
	detailDateRe    = regexp.MustCompile(`(?s)"lbl">Released</div>\s*<div class="val"[^>]*>([^<]+)<`)
)

type detailData struct {
	title       string
	description string
	tags        []string
	performers  []string
	date        time.Time
}

func parseDetailPage(body []byte) detailData {
	var d detailData
	page := string(body)

	if m := detailTitleRe.FindStringSubmatch(page); m != nil {
		d.title = strings.TrimSpace(html.UnescapeString(m[1]))
	}

	if m := detailDescRe.FindStringSubmatch(page); m != nil {
		d.description = strings.TrimSpace(html.UnescapeString(m[1]))
	}

	for _, m := range catTagRe.FindAllStringSubmatch(page, -1) {
		tag := strings.TrimSpace(html.UnescapeString(m[1]))
		if tag != "" {
			d.tags = append(d.tags, tag)
		}
	}

	if m := starringBlockRe.FindStringSubmatch(page); m != nil {
		for _, pm := range performerRe.FindAllStringSubmatch(m[1], -1) {
			name := strings.TrimSpace(html.UnescapeString(pm[1]))
			if name != "" {
				d.performers = append(d.performers, name)
			}
		}
	}

	if m := detailDateRe.FindStringSubmatch(page); m != nil {
		raw := strings.TrimSpace(m[1])
		if t, err := time.Parse("January 2, 2006", raw); err == nil {
			d.date = t.UTC()
		}
	}

	return d
}

func (s *Scraper) fetchDetail(ctx context.Context, item workItem, studioURL string, delay time.Duration) (models.Scene, error) {
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return models.Scene{}, ctx.Err()
		}
	}

	scene := models.Scene{
		ID:         item.slug,
		SiteID:     siteID,
		StudioURL:  studioURL,
		URL:        item.url,
		Title:      item.title,
		Date:       item.date,
		Performers: item.performers,
		Thumbnail:  item.thumb,
		Studio:     "Jules Jordan",
		Series:     item.series,
		ScrapedAt:  time.Now().UTC(),
	}

	body, err := s.fetchPage(ctx, item.url)
	if err != nil {
		return models.Scene{}, fmt.Errorf("detail %s: %w", item.slug, err)
	}

	detail := parseDetailPage(body)
	if detail.title != "" {
		scene.Title = detail.title
	}
	scene.Description = detail.description
	scene.Tags = detail.tags
	if len(detail.performers) > 0 {
		scene.Performers = detail.performers
	}
	if !detail.date.IsZero() {
		scene.Date = detail.date
	}

	return scene, nil
}

// --- parsing: DVD listing ---

var (
	dvdEntryRe = regexp.MustCompile(`(?s)<a href="([^"]*)"[^>]*class="dvd-listing-card"[^>]*>.*?class="dvd-listing-name"[^>]*>([^<]+)<`)
)

func parseDVDListingPage(body []byte) []dvdEntry {
	var entries []dvdEntry
	for _, m := range dvdEntryRe.FindAllStringSubmatch(string(body), -1) {
		entries = append(entries, dvdEntry{
			url:  m[1],
			name: strings.TrimSpace(html.UnescapeString(m[2])),
		})
	}
	return entries
}

func extractDVDSceneURLs(body []byte) []string {
	seen := map[string]bool{}
	var urls []string
	for _, m := range sceneURLRe.FindAllSubmatch(body, -1) {
		u := string(m[1])
		if !seen[u] {
			seen[u] = true
			urls = append(urls, u)
		}
	}
	return urls
}

// --- HTTP ---

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
