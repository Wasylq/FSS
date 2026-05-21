package cherrypimpsutil

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

	dvdCardStartRe = regexp.MustCompile(`<div class="item-update[^"]*item-model[^"]*col">`)
	dvdLinkRe      = regexp.MustCompile(`<a\s+href="([^"]+/dvds/[^"]+\.html)"`)
	dvdPageRe      = regexp.MustCompile(`dvds_page_(\d+)\.html`)

	seriesSlugRe   = regexp.MustCompile(`/series/([^_/.]+)`)
	categorySlugRe = regexp.MustCompile(`/categories/([^_/.]+)`)
	modelSlugRe    = regexp.MustCompile(`/models/([^_/.]+?)(?:\.html)?$`)
	dvdSlugRe      = regexp.MustCompile(`/dvds/([^_/.]+?)(?:\.html)?$`)
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
			item.title = html.UnescapeString(strings.TrimSpace(sm[1]))
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
					item.duration = parseutil.ParseDurationColon(sm[1])
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
	modeDVD
	modeDVDListing
)

type listingConfig struct {
	mode listingMode
	slug string
}

func parseStudioURL(studioURL string) listingConfig {
	if m := modelSlugRe.FindStringSubmatch(studioURL); m != nil {
		return listingConfig{mode: modeModel, slug: m[1]}
	}
	if m := dvdSlugRe.FindStringSubmatch(studioURL); m != nil {
		if m[1] == "dvds" {
			return listingConfig{mode: modeDVDListing}
		}
		return listingConfig{mode: modeDVD, slug: m[1]}
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

	switch lc.mode {
	case modeModel:
		s.scrapeSinglePage(ctx, studioURL, opts, out, now)
		return
	case modeDVD:
		s.scrapeSinglePage(ctx, studioURL, opts, out, now)
		return
	case modeDVDListing:
		s.scrapeDVDListing(ctx, opts, out, now)
		return
	}

	s.scrapeListingPages(ctx, lc, opts, out, now)
}

func (s *Scraper) scrapeSinglePage(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult, now time.Time) {
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

func parseDVDCards(body []byte) []string {
	page := string(body)
	starts := dvdCardStartRe.FindAllStringIndex(page, -1)
	seen := make(map[string]bool, len(starts))
	var urls []string
	for _, loc := range starts {
		rest := page[loc[0]:]
		endIdx := strings.Index(rest, cardEnd)
		if endIdx < 0 {
			continue
		}
		block := rest[:endIdx]
		if m := dvdLinkRe.FindStringSubmatch(block); m != nil {
			u := m[1]
			if !seen[u] {
				seen[u] = true
				urls = append(urls, u)
			}
		}
	}
	return urls
}

func estimateDVDTotal(body []byte) int {
	max := 1
	for _, m := range dvdPageRe.FindAllSubmatch(body, -1) {
		n, _ := strconv.Atoi(string(m[1]))
		if n > max {
			max = n
		}
	}
	return max
}

func (s *Scraper) scrapeDVDListing(ctx context.Context, opts scraper.ListOpts, out chan<- scraper.SceneResult, now time.Time) {
	var allDVDs []string
	totalPages := 1

	for page := 1; page <= totalPages; page++ {
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

		var pageURL string
		if page == 1 {
			pageURL = s.cfg.SiteBase + "/dvds/dvds.html"
		} else {
			pageURL = fmt.Sprintf("%s/dvds/dvds_page_%d.html", s.cfg.SiteBase, page)
		}

		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("dvd listing page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		dvds := parseDVDCards(body)
		if len(dvds) == 0 {
			break
		}
		allDVDs = append(allDVDs, dvds...)

		if page == 1 {
			totalPages = estimateDVDTotal(body)
		}
	}

	if len(allDVDs) == 0 {
		return
	}

	for i, dvdURL := range allDVDs {
		if ctx.Err() != nil {
			return
		}
		if i > 0 && opts.Delay > 0 {
			select {
			case <-time.After(opts.Delay):
			case <-ctx.Done():
				return
			}
		}

		body, err := s.fetchPage(ctx, dvdURL)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("dvd %s: %w", dvdURL, err)):
			case <-ctx.Done():
			}
			return
		}

		scenes := parseListingPage(body)
		if i == 0 {
			select {
			case out <- scraper.Progress(len(scenes) * len(allDVDs)):
			case <-ctx.Done():
				return
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
		URL:     url,
		Headers: httpx.BrowserHeaders(httpx.UserAgentChrome),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}
