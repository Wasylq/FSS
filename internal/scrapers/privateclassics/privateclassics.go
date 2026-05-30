// Package privateclassics scrapes privateclassics.com — the Private
// network's vintage / pre-2010 archive. Uses its own CMS, separate from
// both the modern private.com layout (`<li class="card">`) and the
// privateblack/privatecastings variant (`<li class="col-…">`).
//
// All paths live under a locale prefix `/en/` (also `/es/`, `/de/`, etc.
// exist but we always scrape /en/ for consistency).
//
// Card markup:
//
//	<li class="site-movies">
//	  <article class="content video scene ">
//	    <figure>
//	      <a href="https://www.privateclassics.com/en/scene/{Slug-With-Caps}/{id}">
//	        <img data-src="https://pclassics77.st-content.com/…/contentthumbs/{n}.jpg?secure=…"
//	             class="img-responsive lazyload" alt="…" title="…">
//	      </a>
//	    </figure>
//	    <div class="content-text">
//	      <h1>
//	        <a href="https://www.privateclassics.com/en/scene/{Slug}/{id}">Title</a>
//	      </h1>
//	      <ul class="list-models">
//	        <li><a href="https://www.privateclassics.com/en/pornstar/{n}-{slug}">Name</a></li>
//	      </ul>
//	      <div class="scene-votes">…</div>
//	    </div>
//	  </article>
//	</li>
//
// Pagination: `/en/scenes/{N}/` (path-based, trailing slash). Past-end
// pages return zero `<article class="content video scene">` cards.
//
// No date is shown on the listing card; `Scene.Date` stays zero. The
// listing is sorted newest-first so `KnownIDs` early-stop still works.
// The thumbnail uses `data-src` (lazyload) not `src`.
package privateclassics

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

const (
	defaultBase = "https://www.privateclassics.com"
	scraperID   = "privateclassics"
	studioName  = "Private"
	seriesName  = "Private Classics"
)

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

func (s *Scraper) ID() string { return scraperID }
func (s *Scraper) Patterns() []string {
	return []string{
		"privateclassics.com/",
		"privateclassics.com/en/scenes",
		"privateclassics.com/en/scenes/{N}/",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?privateclassics\.com(?:/|$)`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	// Note the trailing space before the closing `"` — that's how the
	// site emits the class attribute and the strict match keeps us from
	// false-positiving on `class="scene"` strings elsewhere in the page.
	cardStartRe = regexp.MustCompile(`<article class="content video scene `)
	// Scene URL — `/en/scene/{Slug-With-Caps}/{id}` on privateclassics.com.
	// The slug contains mixed case (Liz-Honey-…), so we accept any non-`/`
	// chars; ID is the numeric tail.
	sceneURLRe = regexp.MustCompile(
		`href="(https?://(?:www\.)?privateclassics\.com/en/scene/[^"]+/(\d+))"`,
	)
	// Title — `<h1><a href="…/en/scene/…">Title</a></h1>`.
	titleH1Re = regexp.MustCompile(`(?s)<h1>\s*<a[^>]+href="[^"]+/en/scene/[^"]+/\d+"[^>]*>\s*([^<]+?)\s*</a>\s*</h1>`)
	// Performer link — `<a href="/en/pornstar/{N}-{slug}">Name</a>` inside
	// `<ul class="list-models">`.
	performerRe = regexp.MustCompile(
		`<a[^>]+href="[^"]*/en/pornstar/\d+-[^"]+"[^>]*>\s*([^<]+?)\s*</a>`,
	)
	// Thumbnail — `<img data-src="…">` (lazyload).
	thumbRe = regexp.MustCompile(`<img[^>]+data-src="(https?://pclassics77\.st-content\.com/[^"]+)"`)
	// Largest /en/scenes/{N}/ page number in the pagination block.
	pageLinkRe = regexp.MustCompile(`href="[^"]*/en/scenes/(\d+)/?"`)
)

type sceneItem struct {
	id         string
	url        string
	title      string
	performers []string
	thumb      string
}

func parseListing(body []byte) []sceneItem {
	page := string(body)
	starts := cardStartRe.FindAllStringIndex(page, -1)
	items := make([]sceneItem, 0, len(starts))
	seen := make(map[string]bool, len(starts))

	for i, loc := range starts {
		end := len(page)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		block := page[loc[0]:end]

		var item sceneItem
		if m := sceneURLRe.FindStringSubmatch(block); m != nil {
			item.url = m[1]
			item.id = m[2]
		}
		if item.id == "" || seen[item.id] {
			continue
		}
		seen[item.id] = true

		if m := titleH1Re.FindStringSubmatch(block); m != nil {
			item.title = html.UnescapeString(strings.TrimSpace(m[1]))
		}
		for _, pm := range performerRe.FindAllStringSubmatch(block, -1) {
			name := html.UnescapeString(strings.TrimSpace(pm[1]))
			if name != "" {
				item.performers = append(item.performers, name)
			}
		}
		item.performers = dedupStrings(item.performers)
		if m := thumbRe.FindStringSubmatch(block); m != nil {
			item.thumb = m[1]
		}

		items = append(items, item)
	}
	return items
}

func estimateTotal(body []byte, perPage int) int {
	maxPage := 1
	for _, m := range pageLinkRe.FindAllSubmatch(body, -1) {
		n, _ := strconv.Atoi(string(m[1]))
		if n > maxPage {
			maxPage = n
		}
	}
	return maxPage * perPage
}

func (s *Scraper) listingURL(page int) string {
	if page <= 1 {
		return s.base + "/en/scenes/"
	}
	return fmt.Sprintf("%s/en/scenes/%d/", s.base, page)
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)
	scraper.Debugf(1, "privateclassics: scraping full catalog")

	now := time.Now().UTC()
	firstPage := true
	scraper.Paginate(ctx, opts, "privateclassics", out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := s.listingURL(page)
		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}
		items := parseListing(body)
		var total int
		if firstPage {
			total = estimateTotal(body, len(items))
			firstPage = false
		}
		scenes := make([]models.Scene, len(items))
		for i, item := range items {
			scenes[i] = s.toScene(item, studioURL, now)
		}
		return scraper.PageResult{Scenes: scenes, Total: total}, nil
	})
}

func (s *Scraper) toScene(item sceneItem, studioURL string, now time.Time) models.Scene {
	return models.Scene{
		ID:         item.id,
		SiteID:     scraperID,
		StudioURL:  studioURL,
		Title:      item.title,
		URL:        item.url,
		Thumbnail:  item.thumb,
		Performers: item.performers,
		Studio:     studioName,
		Series:     seriesName,
		ScrapedAt:  now,
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

func dedupStrings(in []string) []string {
	if len(in) <= 1 {
		return in
	}
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
