// Package fittingroom scrapes fitting-room.com, a standalone lingerie try-on
// studio running the Kernel Video Sharing (KVS) tube CMS.
//
// The public listing at /videos_list.php is paginated, but NOT via the
// `-N.php` URLs shown in the markup (those are AJAX-only and 404 on a direct
// GET) nor via `?page=N` (which the CMS ignores and serves page 1). Real
// pagination uses the KVS async block endpoint:
//
//	/videos_list.php?mode=async&function=get_block&block_id=list_videos_common_videos_list&sort_by=post_date&from=N
//
// Cards carry the scene ID, title ("Model | Title"), performer, duration and
// thumbnail, but only a relative "added" date. The absolute release date lives
// in the detail page's schema.org VideoObject JSON-LD (`uploadDate`), so each
// scene's detail page is fetched (via a small worker pool) to recover it.
package fittingroom

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

const (
	siteID       = "fittingroom"
	studio       = "Fitting-Room"
	base         = "https://www.fitting-room.com"
	pageSize     = 12
	blockID      = "list_videos_common_videos_list"
	detailWorker = 6
)

// Scraper implements scraper.StudioScraper for fitting-room.com.
type Scraper struct {
	client *http.Client
	base   string // overridable in tests
}

// New constructs a Fitting-Room scraper with default settings.
func New() *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second), base: base}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		"fitting-room.com",
		"fitting-room.com/videos_list.php",
	}
}

var matchRe = regexp.MustCompile(`(?i)\bfitting-room\.com\b`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		cards, err := s.fetchPage(ctx, page)
		if err != nil {
			return scraper.PageResult{}, err
		}
		if len(cards) == 0 {
			return scraper.PageResult{Done: true}, nil
		}

		scenes := s.buildScenes(ctx, studioURL, cards, now, out)

		// A short page is the last page. If every card's detail fetch failed
		// we still have raw cards, so keep walking (Continue) but signal the
		// true end via Done.
		return scraper.PageResult{
			Scenes:   scenes,
			Done:     len(cards) < pageSize,
			Continue: len(scenes) == 0,
		}, nil
	})
}

// listURL builds the KVS async block URL for a 1-based page.
func (s *Scraper) listURL(page int) string {
	return fmt.Sprintf(
		"%s/videos_list.php?mode=async&function=get_block&block_id=%s&sort_by=post_date&from=%d",
		s.base, blockID, page)
}

func (s *Scraper) fetchPage(ctx context.Context, page int) ([]card, error) {
	u := s.listURL(page)
	scraper.Debugf(1, "%s: fetching listing page %d", siteID, page)
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     u,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading page %d: %w", page, err)
	}
	cards := parseCards(body)
	scraper.Debugf(1, "%s: page %d parsed %d cards", siteID, page, len(cards))
	return cards, nil
}

// ---- card parsing ----

type card struct {
	id        string
	url       string // absolute detail URL
	title     string
	model     string
	duration  int
	thumbnail string
}

var (
	cardRe = regexp.MustCompile(`(?s)<a href="([^"]*?/video/(\d+)/[^"]*?)"\s+title="([^"]*)"[^>]*>` +
		`.*?data-original="([^"]*)"` +
		`.*?<div class="duration">([^<]*)</div>` +
		`.*?<div class="model">(.*?)</div>`)
	tagRe = regexp.MustCompile(`<[^>]+>`)
)

func parseCards(body []byte) []card {
	matches := cardRe.FindAllSubmatch(body, -1)
	cards := make([]card, 0, len(matches))
	for _, m := range matches {
		titleAttr := html.UnescapeString(strings.TrimSpace(string(m[3])))
		model, title := splitTitle(titleAttr)

		modelDiv := strings.TrimSpace(html.UnescapeString(tagRe.ReplaceAllString(string(m[6]), "")))
		if modelDiv != "" {
			model = modelDiv
		}

		cards = append(cards, card{
			id:        string(m[2]),
			url:       string(m[1]),
			title:     title,
			model:     model,
			duration:  parseutil.ParseDurationColon(strings.TrimSpace(string(m[5]))),
			thumbnail: string(m[4]),
		})
	}
	return cards
}

// splitTitle splits a card's "Model | Title" attribute into its parts. If
// there is no " | " separator the whole string is treated as the title.
func splitTitle(s string) (model, title string) {
	if i := strings.Index(s, " | "); i >= 0 {
		return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+3:])
	}
	return "", s
}

// ---- detail fetch (absolute date) ----

func (s *Scraper) buildScenes(ctx context.Context, studioURL string, cards []card, now time.Time, out chan<- scraper.SceneResult) []models.Scene {
	scraper.Debugf(1, "%s: fetching %d detail pages with %d workers", siteID, len(cards), detailWorker)

	results := make([]*models.Scene, len(cards))
	jobs := make(chan int)

	var wg sync.WaitGroup
	for w := 0; w < detailWorker; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				if ctx.Err() != nil {
					return
				}
				c := cards[idx]
				date, err := s.fetchDetailDate(ctx, c.url)
				if err != nil {
					if ctx.Err() == nil {
						select {
						case out <- scraper.Error(fmt.Errorf("detail %s: %w", c.id, err)):
						case <-ctx.Done():
						}
					}
					continue
				}
				sc := toScene(studioURL, c, date, now)
				results[idx] = &sc
			}
		}()
	}

	for i := range cards {
		select {
		case jobs <- i:
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return collect(results)
		}
	}
	close(jobs)
	wg.Wait()
	return collect(results)
}

func collect(results []*models.Scene) []models.Scene {
	scenes := make([]models.Scene, 0, len(results))
	for _, sc := range results {
		if sc != nil {
			scenes = append(scenes, *sc)
		}
	}
	return scenes
}

func (s *Scraper) fetchDetailDate(ctx context.Context, url string) (time.Time, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     url,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return time.Time{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return time.Time{}, fmt.Errorf("reading detail: %w", err)
	}

	vo := parseutil.ExtractVideoObject(body)
	if vo == nil {
		return time.Time{}, fmt.Errorf("no VideoObject JSON-LD")
	}
	raw := vo.UploadDate
	if raw == "" {
		raw = vo.DatePublished
	}
	if raw == "" {
		return time.Time{}, fmt.Errorf("no upload date in JSON-LD")
	}
	t, err := parseutil.TryParseDate(raw, time.RFC3339, "2006-01-02")
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing upload date %q: %w", raw, err)
	}
	return t.UTC(), nil
}

func toScene(studioURL string, c card, date, now time.Time) models.Scene {
	sc := models.Scene{
		ID:        c.id,
		SiteID:    siteID,
		StudioURL: studioURL,
		Title:     c.title,
		URL:       c.url,
		Date:      date,
		Duration:  c.duration,
		Thumbnail: c.thumbnail,
		Studio:    studio,
		ScrapedAt: now,
	}
	if c.model != "" {
		sc.Performers = []string{c.model}
	}
	return sc
}
