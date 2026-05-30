// Package goldwinpass scrapes goldwinpass.com — the one Extreme Movie Pass
// sister site that uses an older Elevated X CMS template instead of the
// `modelfeature`/`set-target-` markup that the rest of the network uses.
//
// Why this is a standalone scraper, not a row in the extrememoviepass table:
//
//   - Different card wrapper: `<div class="th" data-setid="{id}">`
//     (the others use `<div class="modelfeature ... grabthis">`).
//   - Different pagination form: `/tour/updates/page_{N}.html` (the others use
//     `/tour/categories/movies/{N}/latest/`).
//   - Different field layout: `<p class="title"><a title="…">Title</a></p>`,
//     `<span class="date">Puplished on: <b>05/19/2026</b></span>`,
//     `<span class="time">28&nbsp;minute(s)&nbsp;Movie</span>`.
//
// A shared `elevatedxoldutil` might make sense once a 6th `data-setid` site
// shows up — but in practice every site in that family has a different card
// wrapper class (`update_details` / `latestUpdateB` / `videoBlock` / now
// `th`), so a util would just be six regex sets in a trench coat. Standalone
// is cleaner until two sites genuinely share a wrapper.
//
// Like the rest of the Extreme Movie Pass family, there are no public scene
// detail pages — every anchor goes to `https://join.goldwinpass.com/signup/
// signup.php?nats=…`. All metadata is lifted from the listing card; scene URL
// is synthesised as `{base}/tour/#scene-{id}`.
package goldwinpass

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
	"github.com/Wasylq/FSS/scraper"
)

const defaultBase = "https://www.goldwinpass.com"

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

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return "goldwinpass" }

func (s *Scraper) Patterns() []string {
	return []string{"goldwinpass.com", "goldwinpass.com/tour/", "goldwinpass.com/tour/updates/page_{N}.html"}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?goldwinpass\.com`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// Card / pagination regexes.
var (
	cardStartRe = regexp.MustCompile(`<div class="th"\s+data-setid="(\d+)"`)
	titleRe     = regexp.MustCompile(`(?s)<p class="title">.*?<a[^>]+title="([^"]+)"`)
	dateRe      = regexp.MustCompile(`(?s)<span class="date">[^<]*<b>(\d{2}/\d{2}/\d{4})</b>`)
	// "28&nbsp;minute(s)&nbsp;Movie" — minutes-only duration.
	durationRe = regexp.MustCompile(`(\d+)\s*(?:&nbsp;|\s)*minute\(s\)`)
	// Thumbnail can be relative; we resolve below.
	thumbRe   = regexp.MustCompile(`<img[^>]+class="[^"]*update_thumb[^"]*"[^>]+src="([^"]+)"`)
	maxPageRe = regexp.MustCompile(`/tour/updates/page_(\d+)\.html`)
)

type sceneItem struct {
	id       string
	title    string
	thumb    string
	date     time.Time
	duration int // seconds
}

// parseListing slices the page by `data-setid` card starts. Cards that appear
// twice on a page (e.g. "More Updates" sidebar) are deduped.
func parseListing(body []byte) []sceneItem {
	page := string(body)
	starts := cardStartRe.FindAllStringSubmatchIndex(page, -1)
	items := make([]sceneItem, 0, len(starts))
	seen := make(map[string]bool, len(starts))

	for i, loc := range starts {
		id := page[loc[2]:loc[3]]
		if seen[id] {
			continue
		}
		seen[id] = true

		// Slice to next card start (or end of page).
		end := len(page)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		block := page[loc[0]:end]

		item := sceneItem{id: id}
		if m := titleRe.FindStringSubmatch(block); m != nil {
			item.title = html.UnescapeString(strings.TrimSpace(m[1]))
		}
		if m := thumbRe.FindStringSubmatch(block); m != nil {
			item.thumb = m[1]
		}
		if m := dateRe.FindStringSubmatch(block); m != nil {
			if d, err := time.Parse("01/02/2006", m[1]); err == nil {
				item.date = d.UTC()
			}
		}
		if m := durationRe.FindStringSubmatch(block); m != nil {
			mins, _ := strconv.Atoi(m[1])
			item.duration = mins * 60
		}

		items = append(items, item)
	}
	return items
}

func estimateTotal(body []byte, perPage int) int {
	maxPage := 1
	for _, m := range maxPageRe.FindAllSubmatch(body, -1) {
		n, _ := strconv.Atoi(string(m[1]))
		if n > maxPage {
			maxPage = n
		}
	}
	return maxPage * perPage
}

func (s *Scraper) listingURL(page int) string {
	return fmt.Sprintf("%s/tour/updates/page_%d.html", s.base, page)
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)
	scraper.Debugf(1, "goldwinpass: scraping full catalog")

	now := time.Now().UTC()
	sentTotal := false
	scraper.Paginate(ctx, opts, "goldwinpass", out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		body, err := s.fetchPage(ctx, s.listingURL(page))
		if err != nil {
			return scraper.PageResult{}, err
		}

		items := parseListing(body)

		var total int
		if !sentTotal {
			sentTotal = true
			total = estimateTotal(body, len(items))
		}

		scenes := make([]models.Scene, len(items))
		for i, item := range items {
			scenes[i] = item.toScene(s.base, now)
		}
		return scraper.PageResult{Scenes: scenes, Total: total}, nil
	})
}

func (item sceneItem) toScene(base string, now time.Time) models.Scene {
	thumb := item.thumb
	// Listing thumbs are relative paths like `content/gwp-foo/0.jpg`.
	if thumb != "" && !strings.HasPrefix(thumb, "http") {
		thumb = base + "/tour/" + strings.TrimPrefix(thumb, "/")
	}
	return models.Scene{
		ID:        item.id,
		SiteID:    "goldwinpass",
		StudioURL: base,
		Title:     item.title,
		URL:       fmt.Sprintf("%s/tour/#scene-%s", base, item.id),
		Thumbnail: thumb,
		Date:      item.date,
		Duration:  item.duration,
		Studio:    "GoldwinPass",
		ScrapedAt: now,
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
