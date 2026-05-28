// Package darkreachmodernutil scrapes Darkreach Communications sites running
// the "modern" Bootstrap-grid template (mybestsexlife, givingahandjob,
// erotiquetvlive). Listing-only — these sites have public detail pages but
// the listing card already carries title, date, performers, and thumb, so we
// skip the per-scene fetch for speed.
//
// Detection signals:
//
//   - NATS `tourhelper` + `elx_styles.css`.
//   - Card wrapper `<div class="item item-update item-video">` — note the
//     leading `item` class.
//   - Pagination: `{base}/categories/movies_{N}_d.html` (most sites use no
//     `/tour/` prefix; erotiquetvlive uses `/tour/categories/...`).
//   - Detail URL form: `/trailers/{MixedCaseSlug}.html` — case-sensitive slug.
//   - Listing-side date in a `more-info-div` block as
//     `<i class="fa fa-calendar"></i> Mon DD, YYYY`.
//   - Thumb via the lazy-load `src0_1x="..."` attribute on the card image.
//
// Sample card:
//
//	<div class="item item-update item-video">
//	  <div class="img-div">
//	    <a href=".../trailers/MyBestSexLifeFoo.html" title="…">
//	      <img id="set-target-269" class="update_thumb thumbs stdimage"
//	           src0_1x="/content//contentthumbs/05/37/537-1x.jpg" />
//	    </a>
//	  </div>
//	  <div class="content-div">
//	    <h4><a href=".../trailers/MyBestSexLifeFoo.html" title="…">Foo</a></h4>
//	    <div class="more-info-div">
//	      <i class="fa fa-calendar"></i> Jan 23, 2026
//	    </div>
//	  </div>
//	</div>
package darkreachmodernutil

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

// SiteConfig describes one Darkreach Modern site.
type SiteConfig struct {
	ID       string
	SiteBase string // "https://www.mybestsexlife.com" — no trailing slash
	Studio   string
	// TourPrefix is "/tour" for sites whose listings are rooted at /tour/
	// (e.g. erotiquetvlive), or "" for sites that serve listings at the bare
	// `/categories/movies_{N}_d.html` path (mybestsexlife, givingahandjob).
	TourPrefix string
	Patterns   []string
	MatchRe    *regexp.Regexp
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

// Card-parsing regexes.
//
// We anchor cards by the inner `item-update item-video` token because the
// wrapper class starts with `item ` (different from Adult Doorway's
// `item-update no-overlay ...`). Using `item-update item-video` is more
// specific than just `item-update` and avoids false hits on other components.
var (
	cardStartRe   = regexp.MustCompile(`<div class="(?:item\s+)?item-update[^"]*item-video[^"]*"`)
	titleAnchorRe = regexp.MustCompile(`(?s)<h4>\s*<a[^>]+href="([^"]+)"[^>]*title="([^"]+)"`)
	thumbLazyRe   = regexp.MustCompile(`src0_1x="([^"]+)"`)
	// Detail URL slug is case-sensitive (TitleCase / MixedCase).
	slugFromURLRe = regexp.MustCompile(`/trailers/([A-Za-z0-9][A-Za-z0-9_-]*)\.html`)
	// `<i class="fa fa-calendar"></i> Jan 23, 2026`
	listingDateRe = regexp.MustCompile(`fa-calendar"[^>]*></i>\s*([A-Z][a-z]{2}\s+\d{1,2},\s+\d{4})`)
	// Pagination `_(\d+)_d.html` (max-page in the URLs).
	maxPageRe = regexp.MustCompile(`movies_(\d+)_d\.html`)
)

type sceneItem struct {
	id    string // slug from /trailers/{slug}.html
	title string
	url   string
	thumb string
	date  time.Time
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
		if m := titleAnchorRe.FindStringSubmatch(block); m != nil {
			item.url = m[1]
			item.title = html.UnescapeString(strings.TrimSpace(m[2]))
			if slug := slugFromURLRe.FindStringSubmatch(item.url); slug != nil {
				item.id = slug[1]
			}
		}
		if item.id == "" || seen[item.id] {
			continue
		}
		seen[item.id] = true

		if m := thumbLazyRe.FindStringSubmatch(block); m != nil {
			item.thumb = m[1]
		}
		if m := listingDateRe.FindStringSubmatch(block); m != nil {
			if d, err := time.Parse("Jan 2, 2006", m[1]); err == nil {
				item.date = d.UTC()
			}
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
	return fmt.Sprintf("%s%s/categories/movies_%d_d.html", s.cfg.SiteBase, s.cfg.TourPrefix, page)
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)
	scraper.Debugf(1, "%s: scraping full catalog", s.cfg.ID)

	now := time.Now().UTC()
	sentTotal := false

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

		pageURL := s.listingURL(page)
		scraper.Debugf(1, "%s: fetching page %d", s.cfg.ID, page)

		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		items := parseListing(body)
		if len(items) == 0 {
			scraper.Debugf(1, "%s: page %d empty, stopping", s.cfg.ID, page)
			return
		}

		if !sentTotal {
			total := estimateTotal(body, len(items))
			scraper.Debugf(1, "%s: %d total scenes (estimated)", s.cfg.ID, total)
			if total > 0 {
				select {
				case out <- scraper.Progress(total):
				case <-ctx.Done():
					return
				}
			}
			sentTotal = true
		}

		for _, item := range items {
			if opts.KnownIDs[item.id] {
				scraper.Debugf(1, "%s: hit known ID %s, stopping early", s.cfg.ID, item.id)
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
	thumb := item.thumb
	if thumb != "" && !strings.HasPrefix(thumb, "http") {
		// Strip leading slash to dedupe `//content` → `/content`.
		thumb = siteBase + "/" + strings.TrimLeft(thumb, "/")
	}
	return models.Scene{
		ID:        item.id,
		SiteID:    siteID,
		StudioURL: siteBase,
		Title:     item.title,
		URL:       url,
		Thumbnail: thumb,
		Date:      item.date,
		Studio:    studio,
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
