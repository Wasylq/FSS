// Package kocompany scrapes the KO Company network — a Japanese gay JAV
// studio whose 15 sub-labels are all served by ko-video.com running an
// EC-CUBE storefront. Each sub-label is reachable via a label filter
// (`?label={N}`) or, in one case, a maker filter (`?maker={N}`) on
// `/products/list.php`. The package registers one scraper per stashdb
// sub-label.
//
// Card markup:
//
//	<ul class="item_list">
//	  <li>
//	    <a href="/products/detail.php?product_code=KBEA390_DVD">
//	      <img src="/upload/save_image/beast/KBEA390_DVD/KBEA390_DVD_A.jpg"
//	           alt="…title…">
//	      <div class="list_tag">
//	        <span class="icon_qpn">クーポン</span>
//	        <span class="icon_shin">NEW</span>
//	      </div>
//	      <span>…title…</span>
//	    </a>
//	  </li>
//	  …
//	</ul>
//
// Fields lifted: the product code (`KBEA390_DVD`) is the stable ID, the
// title is taken from the inner `<span>` (the `<img alt>` and the span
// are usually identical; the span is more reliable when the alt is
// truncated), the thumbnail URL from `<img src>`, and any tag badges
// (`NEW`, `クーポン`) from the list_tag span children. Detail pages
// exist but aren't fetched — every field stash needs (title, ID,
// thumbnail, brand) is already on the listing.
//
// Pagination: `<div class="page_nav_head">` lists numbered `pageno`
// links via a JavaScript form (`fnModeSubmit(”, 'pageno', N)`); the
// same URL form also works as a query string. 20 items per page on the
// default display. Past-end pages return zero `product_code=` matches
// (clean stop signal).
package kocompany

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
	baseURL    = "https://ko-video.com"
	studioName = "KO Company"
)

// SiteConfig describes one KO Company sub-label.
type SiteConfig struct {
	ID       string
	SiteName string // display name for Scene.Series
	// Either LabelID or MakerID is non-zero — the filter applied to
	// `/products/list.php?{filter}={N}`.
	LabelID  int
	MakerID  int
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

var _ scraper.StudioScraper = (*Scraper)(nil)

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
	// Each card is a `<li>` whose anchor's href encodes the product code.
	// We use the entire anchor as the card boundary so the title `<span>`
	// and `<img>` are inside the captured block.
	cardStartRe = regexp.MustCompile(
		`<a\s+href="/products/detail\.php\?product_code=([A-Z][A-Z0-9_]+)"`,
	)
	// Title — the inner `<span>` after the `<div class="list_tag">`.
	// Anchored on `</div>\s*<span>…</span>` so we don't grab the badge
	// spans (icon_qpn / icon_shin) by mistake.
	titleSpanRe = regexp.MustCompile(`(?s)</div>\s*<span>\s*([^<]+?)\s*</span>`)
	// Fallback title: the `<img alt="…">` attribute (used when the
	// `<span>` is missing or empty).
	imgAltRe = regexp.MustCompile(`<img[^>]+alt="([^"]+)"`)
	// Thumbnail: the `<img src="…">` URL — root-relative on the EC-CUBE
	// CMS, absolutised in toScene.
	thumbRe = regexp.MustCompile(`<img[^>]+src="(/[^"]+)"`)
	// Badge tags: `<span class="icon_qpn">クーポン</span>` etc.
	badgeRe = regexp.MustCompile(`<span class="icon_\w+">\s*([^<]+?)\s*</span>`)
	// Pagination: `fnModeSubmit('', 'pageno', N)` references in the
	// page-nav block. The highest N is the last page.
	pageNoRe = regexp.MustCompile(`fnModeSubmit\(\s*['"][^'"]*['"]\s*,\s*['"]pageno['"]\s*,\s*(\d+)\s*\)`)
)

type sceneItem struct {
	id    string
	title string
	thumb string
	tags  []string
}

func parseListing(body []byte) []sceneItem {
	page := string(body)
	starts := cardStartRe.FindAllStringSubmatchIndex(page, -1)
	items := make([]sceneItem, 0, len(starts))
	seen := make(map[string]bool, len(starts))

	for i, loc := range starts {
		end := len(page)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		block := page[loc[0]:end]

		id := page[loc[2]:loc[3]]
		if seen[id] {
			continue
		}
		seen[id] = true

		item := sceneItem{id: id}
		// Title: try the trailing `<span>` first; fall back to the
		// `<img alt>` attribute.
		if m := titleSpanRe.FindStringSubmatch(block); m != nil {
			item.title = html.UnescapeString(strings.TrimSpace(m[1]))
		}
		if item.title == "" {
			if m := imgAltRe.FindStringSubmatch(block); m != nil {
				item.title = html.UnescapeString(strings.TrimSpace(m[1]))
			}
		}
		if m := thumbRe.FindStringSubmatch(block); m != nil {
			item.thumb = m[1]
		}
		for _, bm := range badgeRe.FindAllStringSubmatch(block, -1) {
			tag := strings.TrimSpace(bm[1])
			if tag != "" {
				item.tags = append(item.tags, tag)
			}
		}

		items = append(items, item)
	}
	return items
}

// estimateTotal reads the highest pageno reference from the page-nav
// helper. Returns perPage when no pagination block is present.
func estimateTotal(body []byte, perPage int) int {
	maxPage := 1
	for _, m := range pageNoRe.FindAllSubmatch(body, -1) {
		n, _ := strconv.Atoi(string(m[1]))
		if n > maxPage {
			maxPage = n
		}
	}
	return maxPage * perPage
}

func (s *Scraper) listingURL(page int) string {
	q := ""
	switch {
	case s.cfg.LabelID != 0:
		q = fmt.Sprintf("label=%d", s.cfg.LabelID)
	case s.cfg.MakerID != 0:
		q = fmt.Sprintf("maker=%d", s.cfg.MakerID)
	}
	if page > 1 {
		q += fmt.Sprintf("&pageno=%d", page)
	}
	return baseURL + "/products/list.php?" + q
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)
	scraper.Debugf(1, "kocompany/%s: scraping", s.cfg.ID)

	siteID := "kocompany/" + s.cfg.ID
	now := time.Now().UTC()
	sentTotal := false
	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
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
			scenes[i] = s.toScene(item, studioURL, now)
		}
		return scraper.PageResult{Scenes: scenes, Total: total}, nil
	})
}

func (s *Scraper) toScene(item sceneItem, studioURL string, now time.Time) models.Scene {
	thumb := item.thumb
	if thumb != "" && strings.HasPrefix(thumb, "/") {
		thumb = baseURL + thumb
	}
	return models.Scene{
		ID:        item.id,
		SiteID:    s.cfg.ID,
		StudioURL: studioURL,
		Title:     item.title,
		URL:       baseURL + "/products/detail.php?product_code=" + item.id,
		Thumbnail: thumb,
		Studio:    studioName,
		Series:    s.cfg.SiteName,
		Tags:      item.tags,
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
