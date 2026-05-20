package julesjordanutil

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

const defaultDelay = 500 * time.Millisecond

type TemplateType int

const (
	TemplateJJ      TemplateType = iota // julesjordan.com — new theme
	TemplateClassic                     // manuelferrara.com, girlgirl.com, spermswallowers.com
	TemplateModern                      // theassfactory.com
)

type SiteConfig struct {
	SiteID     string
	Domain     string
	StudioName string
	Template   TemplateType
}

type Scraper struct {
	client *http.Client
	base   string
	Config SiteConfig
}

func NewScraper(cfg SiteConfig) *Scraper {
	return &Scraper{
		client: httpx.NewClient(30 * time.Second),
		base:   "https://www." + cfg.Domain + "/trial",
		Config: cfg,
	}
}

func (s *Scraper) ID() string { return s.Config.SiteID }

func (s *Scraper) Patterns() []string {
	d := s.Config.Domain
	return []string{
		d + "/trial/",
		d + "/trial/categories/movies.html",
		d + "/trial/models/{slug}.html",
		d + "/trial/dvds/dvds.html",
	}
}

func (s *Scraper) MatchesURL(u string) bool {
	d := s.Config.Domain
	return strings.Contains(u, "://"+d) || strings.Contains(u, "://www."+d)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

type workItem struct {
	url        string
	slug       string
	title      string
	performers []string
	date       time.Time
	thumb      string
	series     string
}

type dvdEntry struct {
	url  string
	name string
}

type detailData struct {
	title       string
	description string
	tags        []string
	performers  []string
	date        time.Time
	series      string
}

// --- runner ---

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

		items := s.parseListingCards(body)
		if len(items) == 0 {
			return
		}

		if page == 1 {
			maxPage := s.extractMaxPage(body)
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

	items := s.parseListingCards(body)
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

		dvds := s.parseDVDListing(body)
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

// --- shared regexes ---

var (
	sceneURLRe  = regexp.MustCompile(`href="([^"]*scenes/[^"]*_vids\.html)"`)
	slugRe      = regexp.MustCompile(`/scenes/([^/]+?)_vids\.html`)
	performerRe = regexp.MustCompile(`href="[^"]*models/[^"]*">([^<]+)</a>`)
	thumbRe     = regexp.MustCompile(`<img[^>]*id="set-target-\d+"[^>]*src="([^"]+)"`)
)

// --- template dispatch ---

func (s *Scraper) parseListingCards(body []byte) []workItem {
	switch s.Config.Template {
	case TemplateClassic:
		return parseListingClassic(body, s.base)
	case TemplateModern:
		return parseListingModern(body, s.base)
	default:
		return parseListingJJ(body, s.base)
	}
}

func (s *Scraper) parseDetail(body []byte) detailData {
	switch s.Config.Template {
	case TemplateClassic:
		return parseDetailClassic(body)
	case TemplateModern:
		return parseDetailModern(body)
	default:
		return parseDetailJJ(body)
	}
}

func (s *Scraper) extractMaxPage(body []byte) int {
	switch s.Config.Template {
	case TemplateClassic:
		return extractMaxPageClassic(body)
	case TemplateModern:
		return 0
	default:
		return extractMaxPageJJ(body)
	}
}

func (s *Scraper) parseDVDListing(body []byte) []dvdEntry {
	switch s.Config.Template {
	case TemplateClassic, TemplateModern:
		return parseDVDListingGeneric(body)
	default:
		return parseDVDListingJJ(body)
	}
}

// --- fetchDetail ---

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
		SiteID:     s.Config.SiteID,
		StudioURL:  studioURL,
		URL:        item.url,
		Title:      item.title,
		Date:       item.date,
		Performers: item.performers,
		Thumbnail:  item.thumb,
		Studio:     s.Config.StudioName,
		Series:     item.series,
		ScrapedAt:  time.Now().UTC(),
	}

	body, err := s.fetchPage(ctx, item.url)
	if err != nil {
		return models.Scene{}, fmt.Errorf("detail %s: %w", item.slug, err)
	}

	detail := s.parseDetail(body)
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
	if detail.series != "" {
		scene.Series = detail.series
	}

	return scene, nil
}

// ========== JJ Template ==========

var (
	jjCardRe    = regexp.MustCompile(`class="jj-content-card"`)
	jjTitleRe   = regexp.MustCompile(`class="jj-card-title"[^>]*>([^<]+)<`)
	jjDateRe    = regexp.MustCompile(`class="jj-card-date"[^>]*>Released:\s*([^<]+)<`)
	jjThumbRe   = regexp.MustCompile(`class="jj-thumb-img[^"]*"[^>]*src="([^"]+)"`)
	jjMaxPageRe = regexp.MustCompile(`movies_(\d+)_d\.html`)
)

func parseListingJJ(body []byte, base string) []workItem {
	page := string(body)
	locs := jjCardRe.FindAllStringIndex(page, -1)
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

		if m := jjTitleRe.FindStringSubmatch(block); m != nil {
			item.title = strings.TrimSpace(html.UnescapeString(m[1]))
		}
		for _, m := range performerRe.FindAllStringSubmatch(block, -1) {
			name := strings.TrimSpace(html.UnescapeString(m[1]))
			if name != "" {
				item.performers = append(item.performers, name)
			}
		}
		if m := jjDateRe.FindStringSubmatch(block); m != nil {
			if t, err := time.Parse("January 2, 2006", strings.TrimSpace(m[1])); err == nil {
				item.date = t.UTC()
			}
		}
		if m := jjThumbRe.FindStringSubmatch(block); m != nil {
			item.thumb = m[1]
		}

		items = append(items, item)
	}
	return items
}

func extractMaxPageJJ(body []byte) int {
	var max int
	for _, m := range jjMaxPageRe.FindAllSubmatch(body, -1) {
		if n, _ := strconv.Atoi(string(m[1])); n > max {
			max = n
		}
	}
	return max
}

var (
	jjDetailTitleRe   = regexp.MustCompile(`class="scene-title"[^>]*>([^<]+)<`)
	jjDetailDescRe    = regexp.MustCompile(`(?s)class="scene-desc"[^>]*>(.*?)</div>`)
	jjCatTagRe        = regexp.MustCompile(`class="cat-tag"[^>]*>([^<]+)</a>`)
	jjStarringBlockRe = regexp.MustCompile(`(?s)"lbl">Starring</div>\s*<div class="val"[^>]*>(.*?)</div>`)
	jjDetailDateRe    = regexp.MustCompile(`(?s)"lbl">Released</div>\s*<div class="val"[^>]*>([^<]+)<`)
)

func parseDetailJJ(body []byte) detailData {
	var d detailData
	page := string(body)

	if m := jjDetailTitleRe.FindStringSubmatch(page); m != nil {
		d.title = strings.TrimSpace(html.UnescapeString(m[1]))
	}
	if m := jjDetailDescRe.FindStringSubmatch(page); m != nil {
		d.description = strings.TrimSpace(html.UnescapeString(m[1]))
	}
	for _, m := range jjCatTagRe.FindAllStringSubmatch(page, -1) {
		tag := strings.TrimSpace(html.UnescapeString(m[1]))
		if tag != "" {
			d.tags = append(d.tags, tag)
		}
	}
	if m := jjStarringBlockRe.FindStringSubmatch(page); m != nil {
		for _, pm := range performerRe.FindAllStringSubmatch(m[1], -1) {
			name := strings.TrimSpace(html.UnescapeString(pm[1]))
			if name != "" {
				d.performers = append(d.performers, name)
			}
		}
	}
	if m := jjDetailDateRe.FindStringSubmatch(page); m != nil {
		if t, err := time.Parse("January 2, 2006", strings.TrimSpace(m[1])); err == nil {
			d.date = t.UTC()
		}
	}
	return d
}

// ========== Classic Template (manuelferrara, girlgirl, spermswallowers) ==========

var (
	classicCardRe    = regexp.MustCompile(`class="update_details"`)
	classicTitleRe   = regexp.MustCompile(`(?s)<a[^>]*href="[^"]*scenes/[^"]*_vids\.html"[^>]*>\s*([^<]+?)\s*</a>`)
	classicDateRe    = regexp.MustCompile(`class="[^"]*update_date[^"]*"[^>]*>[^<]*?(\d{2}/\d{2}/\d{4})`)
	classicMaxPageRe = regexp.MustCompile(`Page \d+ of (\d+)`)
)

func parseListingClassic(body []byte, base string) []workItem {
	page := string(body)
	locs := classicCardRe.FindAllStringIndex(page, -1)
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

		if m := classicTitleRe.FindStringSubmatch(block); m != nil {
			item.title = strings.TrimSpace(html.UnescapeString(m[1]))
		}
		for _, m := range performerRe.FindAllStringSubmatch(block, -1) {
			name := strings.TrimSpace(html.UnescapeString(m[1]))
			if name != "" {
				item.performers = append(item.performers, name)
			}
		}
		if m := classicDateRe.FindStringSubmatch(block); m != nil {
			if t, err := time.Parse("01/02/2006", m[1]); err == nil {
				item.date = t.UTC()
			}
		}
		if m := thumbRe.FindStringSubmatch(block); m != nil {
			item.thumb = m[1]
		}

		items = append(items, item)
	}
	return items
}

func extractMaxPageClassic(body []byte) int {
	m := classicMaxPageRe.FindSubmatch(body)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(string(m[1]))
	return n
}

var (
	classicDetailTitleRe = regexp.MustCompile(`(?s)class="title_bar_hilite"[^>]*>(.*?)</span>`)
	classicDetailDescRe  = regexp.MustCompile(`(?s)class="update_description"[^>]*>(.*?)</span>`)
	classicTagRe         = regexp.MustCompile(`<a[^>]*href="[^"]*categories/[^"]*"[^>]*>([^<]+)</a>`)
	classicSeriesRe      = regexp.MustCompile(`(?s)class="update_dvds"[^>]*>.*?<a[^>]*>([^<]+)</a>`)
	modelsBlockRe        = regexp.MustCompile(`(?s)<span class="update_models">(.*?)</span>`)
	tagStripRe           = regexp.MustCompile(`<[^>]+>`)
)

func parseDetailClassic(body []byte) detailData {
	var d detailData
	page := string(body)

	if m := classicDetailTitleRe.FindStringSubmatch(page); m != nil {
		raw := tagStripRe.ReplaceAllString(m[1], "")
		d.title = strings.TrimSpace(html.UnescapeString(raw))
	}
	if m := classicDetailDescRe.FindStringSubmatch(page); m != nil {
		d.description = strings.TrimSpace(html.UnescapeString(m[1]))
	}
	for _, m := range classicTagRe.FindAllStringSubmatch(page, -1) {
		tag := strings.TrimSpace(html.UnescapeString(m[1]))
		if tag != "" {
			d.tags = append(d.tags, tag)
		}
	}
	d.performers = extractScopedPerformers(page)
	if m := classicDateRe.FindStringSubmatch(page); m != nil {
		if t, err := time.Parse("01/02/2006", m[1]); err == nil {
			d.date = t.UTC()
		}
	}
	if m := classicSeriesRe.FindStringSubmatch(page); m != nil {
		d.series = strings.TrimSpace(html.UnescapeString(m[1]))
	}
	return d
}

// ========== Modern Template (theassfactory) ==========

var (
	modernCardRe        = regexp.MustCompile(`class="grid-item"`)
	modernImgAltRe      = regexp.MustCompile(`<img[^>]*id="set-target-\d+"[^>]*alt="([^"]+)"`)
	modernDetailTitleRe = regexp.MustCompile(`class="movie_title"[^>]*>([^<]+)<`)
	modernDescRe        = regexp.MustCompile(`(?s)>Description:</span>\s*(.*?)</div>`)
	modernDateRe        = regexp.MustCompile(`>Date:</span>\s*(\d{4}-\d{2}-\d{2})`)
)

func parseListingModern(body []byte, base string) []workItem {
	page := string(body)
	locs := modernCardRe.FindAllStringIndex(page, -1)
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

		if m := modernImgAltRe.FindStringSubmatch(block); m != nil {
			item.title = strings.TrimSpace(html.UnescapeString(m[1]))
		}
		for _, m := range performerRe.FindAllStringSubmatch(block, -1) {
			name := strings.TrimSpace(html.UnescapeString(m[1]))
			if name != "" {
				item.performers = append(item.performers, name)
			}
		}
		if m := thumbRe.FindStringSubmatch(block); m != nil {
			item.thumb = m[1]
		}

		items = append(items, item)
	}
	return items
}

func parseDetailModern(body []byte) detailData {
	var d detailData
	page := string(body)

	if m := modernDetailTitleRe.FindStringSubmatch(page); m != nil {
		d.title = strings.TrimSpace(html.UnescapeString(m[1]))
	}
	if m := modernDescRe.FindStringSubmatch(page); m != nil {
		d.description = strings.TrimSpace(html.UnescapeString(m[1]))
	}
	for _, m := range classicTagRe.FindAllStringSubmatch(page, -1) {
		tag := strings.TrimSpace(html.UnescapeString(m[1]))
		if tag != "" {
			d.tags = append(d.tags, tag)
		}
	}
	d.performers = extractScopedPerformers(page)
	if m := modernDateRe.FindStringSubmatch(page); m != nil {
		if t, err := time.Parse("2006-01-02", m[1]); err == nil {
			d.date = t.UTC()
		}
	}
	return d
}

// ========== DVD Parsing ==========

var jjDVDRe = regexp.MustCompile(`(?s)<a href="([^"]*)"[^>]*class="dvd-listing-card"[^>]*>.*?class="dvd-listing-name"[^>]*>([^<]+)<`)

func parseDVDListingJJ(body []byte) []dvdEntry {
	var entries []dvdEntry
	for _, m := range jjDVDRe.FindAllStringSubmatch(string(body), -1) {
		entries = append(entries, dvdEntry{
			url:  m[1],
			name: strings.TrimSpace(html.UnescapeString(m[2])),
		})
	}
	return entries
}

var genericDVDRe = regexp.MustCompile(`<a[^>]*href="([^"]*dvds/[^"]+\.html)"[^>]*>([^<]+)</a>`)

func parseDVDListingGeneric(body []byte) []dvdEntry {
	var entries []dvdEntry
	for _, m := range genericDVDRe.FindAllStringSubmatch(string(body), -1) {
		name := strings.TrimSpace(html.UnescapeString(m[2]))
		if name != "" {
			entries = append(entries, dvdEntry{url: m[1], name: name})
		}
	}
	return entries
}

func extractScopedPerformers(page string) []string {
	m := modelsBlockRe.FindStringSubmatch(page)
	if m == nil {
		return nil
	}
	var performers []string
	for _, pm := range performerRe.FindAllStringSubmatch(m[1], -1) {
		name := strings.TrimSpace(html.UnescapeString(pm[1]))
		if name != "" {
			performers = append(performers, name)
		}
	}
	return performers
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
		Headers: httpx.BrowserHeaders(httpx.UserAgentChrome),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}
