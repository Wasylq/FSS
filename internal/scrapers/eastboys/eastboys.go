// Package eastboys scrapes eastboys.com, a standalone gay studio on a custom
// CMS. The public tour listing (/tour/video) exposes scene cards (id, title,
// thumbnail, release date, duration) and each detail page
// (/tour/eastboys-trailer/{id}/{slug}) embeds a schema.org VideoObject in
// JSON-LD plus a "Categories" block, which we use to enrich performers, tags
// and description.
package eastboys

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
	"github.com/Wasylq/FSS/parseutil"
	"github.com/Wasylq/FSS/scraper"
)

const (
	siteID  = "eastboys"
	studio  = "EastBoys"
	base    = "https://www.eastboys.com"
	workers = 6
)

// Scraper implements scraper.StudioScraper for eastboys.com.
type Scraper struct {
	client *http.Client
	base   string // overridable in tests
}

// New constructs an EastBoys scraper.
func New() *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second), base: base}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		"eastboys.com",
		"eastboys.com/tour/video",
		"eastboys.com/tour/eastboys-trailer/{id}/{slug}",
		"eastboys.com/tour/actor-from-eastboys/{id}/{slug}",
		"eastboys.com/tour/categories/{id}/{slug}",
	}
}

var matchRe = regexp.MustCompile(`(?i)^https?://(?:www\.)?eastboys\.com`)

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, opts, out)
	return out, nil
}

func (s *Scraper) run(ctx context.Context, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	maxPage := 0 // discovered from the first page's pagination block

	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		body, err := s.fetchListing(ctx, page)
		if err != nil {
			return scraper.PageResult{}, err
		}

		if page == 1 {
			maxPage = parseMaxPage(body)
			scraper.Debugf(1, "%s: discovered max page %d", siteID, maxPage)
		}

		cards := parseCards(body)
		if len(cards) == 0 {
			return scraper.PageResult{Done: true}, nil
		}

		scenes := s.enrich(ctx, cards, now)

		done := maxPage > 0 && page >= maxPage
		return scraper.PageResult{Scenes: scenes, Done: done}, nil
	})
}

func (s *Scraper) fetchListing(ctx context.Context, page int) ([]byte, error) {
	u := fmt.Sprintf("%s/tour/video?order=newest&page=%d", s.base, page)
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     u,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}

// enrich fetches each card's detail page concurrently to add performers, tags
// and description on top of the listing data. A detail-fetch failure is
// non-fatal: the listing-only scene is kept and the failure is logged.
func (s *Scraper) enrich(ctx context.Context, cards []card, now time.Time) []models.Scene {
	scraper.Debugf(1, "%s: fetching %d details with %d workers", siteID, len(cards), workers)

	scenes := make([]models.Scene, len(cards))
	jobs := make(chan int)

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				scenes[i] = s.buildScene(ctx, cards[i], now)
			}
		}()
	}

producer:
	for i := range cards {
		select {
		case jobs <- i:
		case <-ctx.Done():
			break producer
		}
	}
	close(jobs)
	wg.Wait()

	// Drop any scenes left empty by a cancelled context.
	out := scenes[:0]
	for _, sc := range scenes {
		if sc.ID != "" {
			out = append(out, sc)
		}
	}
	return out
}

func (s *Scraper) buildScene(ctx context.Context, c card, now time.Time) models.Scene {
	sc := models.Scene{
		ID:        c.id,
		SiteID:    siteID,
		StudioURL: s.base,
		Title:     c.title,
		URL:       s.base + c.path,
		Thumbnail: c.thumbnail,
		Duration:  c.duration,
		Date:      c.date,
		Studio:    studio,
		ScrapedAt: now,
	}

	detail, err := s.fetchDetail(ctx, c.path)
	if err != nil {
		scraper.Debugf(1, "%s: detail fetch failed for %s: %v", siteID, c.path, err)
		return sc
	}

	if vo := parseutil.ExtractVideoObject(detail); vo != nil {
		if vo.Description != "" {
			sc.Description = html.UnescapeString(strings.TrimSpace(vo.Description))
		}
		if len(vo.Actors) > 0 {
			sc.Performers = vo.Actors
		}
		if sc.Thumbnail == "" && vo.ThumbnailURL != "" {
			sc.Thumbnail = vo.ThumbnailURL
		}
		if d := parseutil.ParseDurationISO(vo.Duration); d > 0 {
			sc.Duration = d
		}
		if t := parseUploadDate(vo); !t.IsZero() {
			sc.Date = t
		}
	}

	if tags := parseCategories(detail); len(tags) > 0 {
		sc.Tags = tags
	}

	return sc
}

func (s *Scraper) fetchDetail(ctx context.Context, path string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     s.base + path,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}

// ---- parsing ----

type card struct {
	id        string
	path      string
	title     string
	thumbnail string
	date      time.Time
	duration  int
}

var (
	cardSplitRe = regexp.MustCompile(`class="item col-xl-3`)
	cardLinkRe  = regexp.MustCompile(`/tour/eastboys-trailer/(\d+)/([^"?#]+)`)
	cardTitleRe = regexp.MustCompile(`(?s)<h3>.*?<a href="/tour/eastboys-trailer/\d+/[^"]+">([^<]+)</a>\s*</h3>`)
	cardThumbRe = regexp.MustCompile(`class="nahledak"[^>]*src="([^"]+)"`)
	cardDurRe   = regexp.MustCompile(`<li>\s*([\d:]+)\s*Minutes\s*</li>`)
	cardDateRe  = regexp.MustCompile(`<span>\s*(\d{2}-\d{2}-\d{4})\s*</span>`)
	pageNumRe   = regexp.MustCompile(`[?&]page=(\d+)`)
	catRe       = regexp.MustCompile(`class="kat-single"><a href="/tour/categories/\d+/[^"]+">([^<]+)</a>`)
)

// parseCards splits a listing page into scene cards and extracts the
// listing-level fields. Only cards with a trailer link + title are returned.
func parseCards(body []byte) []card {
	idx := cardSplitRe.FindAllIndex(body, -1)
	cards := make([]card, 0, len(idx))
	seen := make(map[string]bool)
	for i, loc := range idx {
		end := len(body)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		chunk := body[loc[0]:end]

		link := cardLinkRe.FindSubmatch(chunk)
		if link == nil {
			continue
		}
		id := string(link[1])
		if seen[id] {
			continue
		}

		title := cardTitleRe.FindSubmatch(chunk)
		if title == nil {
			continue
		}
		seen[id] = true

		c := card{
			id:    id,
			path:  fmt.Sprintf("/tour/eastboys-trailer/%s/%s", id, string(link[2])),
			title: html.UnescapeString(strings.TrimSpace(string(title[1]))),
		}
		if m := cardThumbRe.FindSubmatch(chunk); m != nil {
			c.thumbnail = string(m[1])
		}
		if m := cardDurRe.FindSubmatch(chunk); m != nil {
			c.duration = parseutil.ParseDurationColon(string(m[1]))
		}
		if m := cardDateRe.FindSubmatch(chunk); m != nil {
			if t, err := parseutil.TryParseDate(string(m[1]), "02-01-2006"); err == nil {
				c.date = t
			}
		}
		cards = append(cards, c)
	}
	return cards
}

// parseMaxPage returns the highest page number referenced in the
// gen-pagination2 navigation block, or 0 if none is found.
func parseMaxPage(body []byte) int {
	i := strings.Index(string(body), `gen-pagination2">`)
	if i < 0 {
		return 0
	}
	block := body[i:]
	max := 0
	for _, m := range pageNumRe.FindAllSubmatch(block, -1) {
		if n, err := strconv.Atoi(string(m[1])); err == nil && n > max {
			max = n
		}
	}
	return max
}

// parseCategories extracts the scene's category names from the detail page's
// "Categories" block (kat-single spans).
func parseCategories(body []byte) []string {
	var tags []string
	for _, m := range catRe.FindAllSubmatch(body, -1) {
		name := html.UnescapeString(strings.TrimSpace(string(m[1])))
		if name != "" {
			tags = append(tags, name)
		}
	}
	return tags
}

func parseUploadDate(vo *parseutil.VideoObject) time.Time {
	for _, raw := range []string{vo.UploadDate, vo.DatePublished} {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if t, err := parseutil.TryParseDate(raw, time.RFC3339, "2006-01-02T15:04:05Z07:00", "2006-01-02"); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
