package nookies

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/parseutil"
	"github.com/Wasylq/FSS/scraper"
)

func init() { scraper.Register(New()) }

type Scraper struct {
	client *http.Client
	base   string
}

func New() *Scraper {
	return &Scraper{
		client: httpx.NewClient(30 * time.Second),
		base:   "https://nookies.com",
	}
}

func (s *Scraper) ID() string { return "nookies" }

func (s *Scraper) Patterns() []string {
	return []string{
		"nookies.com/videos",
		"nookies.com/site/{slug}",
		"nookies.com/model/{slug}",
		"nookies.com/tag/{slug}",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?nookies\.com(?:/|$)`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	cardRe      = regexp.MustCompile(`(?s)<div class="video-card">.*?<!-- End video-card-->`)
	cardURLRe   = regexp.MustCompile(`<a href="/video/(\d+)/([^"]+)"`)
	cardThumbRe = regexp.MustCompile(`<img src="([^"]+)" alt="([^"]*)"[^>]*class="main-video-img`)
	cardPerfRe  = regexp.MustCompile(`<a href="/model/[^"]*" class="tag-btn">([^<]+)</a>`)
	cardDateRe  = regexp.MustCompile(`<span class="date">(\d{4}-\d{2}-\d{2})</span>`)
	cardSiteRe  = regexp.MustCompile(`<a href="/site/([^"]+)">`)

	nextPageRe = regexp.MustCompile(`<a[^>]+href="[^"]*\?page=(\d+)"[^>]*>\s*»`)

	detailTitleRe = regexp.MustCompile(`<h1[^>]*>([^<]+)</h1>`)
	detailDescRe  = regexp.MustCompile(`(?s)<h3>Description:</h3>\s*<p>(.*?)</p>`)
	detailDurRe   = regexp.MustCompile(`(?s)<i class="fa-solid fa-video"></i>\s*(\d+:\d{2})`)
	detailTagRe   = regexp.MustCompile(`<a class="pill-link" href="/tag/[^"]*">([^<]+)</a>`)

	modelRe = regexp.MustCompile(`/model/`)
	siteRe  = regexp.MustCompile(`/site/`)
	tagRe   = regexp.MustCompile(`/tag/`)
)

type listItem struct {
	id         string
	url        string
	title      string
	thumbnail  string
	performers []string
	date       time.Time
	subSite    string
}

type detailData struct {
	title       string
	description string
	duration    int
	tags        []string
}

func parseListingPage(body []byte) []listItem {
	cards := cardRe.FindAll(body, -1)
	items := make([]listItem, 0, len(cards))
	for _, card := range cards {
		if it, ok := parseCard(card); ok {
			items = append(items, it)
		}
	}
	return items
}

func parseCard(card []byte) (listItem, bool) {
	m := cardURLRe.FindSubmatch(card)
	if m == nil {
		return listItem{}, false
	}

	it := listItem{
		id:  string(m[1]),
		url: "/video/" + string(m[1]) + "/" + string(m[2]),
	}

	if mThumb := cardThumbRe.FindSubmatch(card); mThumb != nil {
		it.thumbnail = string(mThumb[1])
		it.title = html.UnescapeString(strings.TrimSpace(string(mThumb[2])))
	}

	for _, mPerf := range cardPerfRe.FindAllSubmatch(card, -1) {
		name := strings.TrimSpace(html.UnescapeString(string(mPerf[1])))
		if name != "" {
			it.performers = append(it.performers, name)
		}
	}

	if mDate := cardDateRe.FindSubmatch(card); mDate != nil {
		if t, err := time.Parse("2006-01-02", string(mDate[1])); err == nil {
			it.date = t
		}
	}

	if mSite := cardSiteRe.FindSubmatch(card); mSite != nil {
		it.subSite = string(mSite[1])
	}

	return it, true
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	switch {
	case modelRe.MatchString(studioURL):
		scraper.Debugf(1, "nookies: detected model page")
		s.scrapeSinglePage(ctx, studioURL, opts, out)
	case siteRe.MatchString(studioURL):
		scraper.Debugf(1, "nookies: detected site page")
		s.scrapePaginated(ctx, studioURL, opts, out)
	case tagRe.MatchString(studioURL):
		scraper.Debugf(1, "nookies: detected tag page")
		s.scrapePaginated(ctx, studioURL, opts, out)
	default:
		s.scrapePaginated(ctx, s.base+"/videos", opts, out)
	}
}

func (s *Scraper) scrapeSinglePage(ctx context.Context, pageURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	body, err := s.fetchPage(ctx, pageURL)
	if err != nil {
		select {
		case out <- scraper.Error(err):
		case <-ctx.Done():
		}
		return
	}

	items := parseListingPage(body)
	if len(items) == 0 {
		return
	}

	scraper.Debugf(1, "nookies: found %d scenes on single page", len(items))
	select {
	case out <- scraper.Progress(len(items)):
	case <-ctx.Done():
		return
	}

	now := time.Now().UTC()
	scenes := s.fetchDetails(ctx, items, opts, now)
	for _, sc := range scenes {
		if opts.KnownIDs[sc.ID] {
			scraper.Debugf(1, "nookies: hit known ID, stopping early")
			select {
			case out <- scraper.StoppedEarly():
			case <-ctx.Done():
			}
			return
		}
		select {
		case out <- scraper.Scene(sc):
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scraper) scrapePaginated(ctx context.Context, baseURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, "nookies", out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := baseURL
		if page > 1 {
			if strings.Contains(baseURL, "?") {
				pageURL = fmt.Sprintf("%s&page=%d", baseURL, page)
			} else {
				pageURL = fmt.Sprintf("%s?page=%d", baseURL, page)
			}
		}

		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}

		items := parseListingPage(body)
		if len(items) == 0 {
			return scraper.PageResult{}, nil
		}

		total := estimateTotal(body, len(items))
		scenes := s.fetchDetails(ctx, items, opts, now)
		return scraper.PageResult{
			Scenes: scenes,
			Total:  total,
			Done:   !hasNextPage(body),
		}, nil
	})
}

func estimateTotal(body []byte, perPage int) int {
	if m := nextPageRe.FindSubmatch(body); m == nil {
		return perPage
	}
	maxPage := 1
	for _, m := range regexp.MustCompile(`\?page=(\d+)`).FindAllSubmatch(body, -1) {
		n := 0
		for _, c := range m[1] {
			n = n*10 + int(c-'0')
		}
		if n > maxPage {
			maxPage = n
		}
	}
	return maxPage * perPage
}

func hasNextPage(body []byte) bool {
	return nextPageRe.Match(body)
}

func (s *Scraper) fetchDetails(ctx context.Context, items []listItem, opts scraper.ListOpts, now time.Time) []models.Scene {
	workers := opts.Workers
	if workers <= 0 {
		workers = 4
	}

	scraper.Debugf(1, "nookies: fetching %d details with %d workers", len(items), workers)

	type enriched struct {
		item   listItem
		detail detailData
		err    error
	}

	results := make([]enriched, len(items))
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)

	for i, it := range items {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(idx int, item listItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if opts.Delay > 0 {
				select {
				case <-time.After(opts.Delay):
				case <-ctx.Done():
					return
				}
			}

			detail, err := s.fetchDetail(ctx, s.base+item.url)
			results[idx] = enriched{item: item, detail: detail, err: err}
		}(i, it)
	}
	wg.Wait()

	var scenes []models.Scene
	for _, r := range results {
		if ctx.Err() != nil {
			break
		}
		if r.err != nil || r.item.id == "" {
			continue
		}
		scenes = append(scenes, toScene(s.base, r.item, r.detail, now))
	}
	return scenes
}

func (s *Scraper) fetchDetail(ctx context.Context, rawURL string) (detailData, error) {
	body, err := s.fetchPage(ctx, rawURL)
	if err != nil {
		return detailData{}, err
	}
	return parseDetailPage(body), nil
}

func parseDetailPage(body []byte) detailData {
	var d detailData

	if m := detailTitleRe.FindSubmatch(body); m != nil {
		d.title = html.UnescapeString(strings.TrimSpace(string(m[1])))
	}

	if m := detailDescRe.FindSubmatch(body); m != nil {
		d.description = html.UnescapeString(strings.TrimSpace(string(m[1])))
	}

	if m := detailDurRe.FindSubmatch(body); m != nil {
		d.duration = parseutil.ParseDurationColon(string(m[1]))
	}

	seen := make(map[string]bool)
	for _, m := range detailTagRe.FindAllSubmatch(body, -1) {
		tag := strings.TrimSpace(html.UnescapeString(string(m[1])))
		if tag != "" && !seen[tag] {
			seen[tag] = true
			d.tags = append(d.tags, tag)
		}
	}

	return d
}

func toScene(base string, it listItem, d detailData, now time.Time) models.Scene {
	siteID := "nookies"
	if it.subSite != "" {
		siteID = it.subSite
	}

	title := it.title
	if d.title != "" {
		title = d.title
	}

	return models.Scene{
		ID:          it.id,
		SiteID:      siteID,
		StudioURL:   base,
		Title:       title,
		URL:         base + it.url,
		Thumbnail:   it.thumbnail,
		Date:        it.date,
		Duration:    d.duration,
		Performers:  it.performers,
		Description: d.description,
		Tags:        d.tags,
		ScrapedAt:   now,
	}
}

func (s *Scraper) fetchPage(ctx context.Context, url string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     url,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}
