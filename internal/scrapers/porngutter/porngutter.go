// Package porngutter scrapes the Porn Gutter / SmutPuppet network of
// "bonus sites". 19 of the network's sister domains are listed under one
// scraper ID — they share a single catalogue (`/updates/`) and each scene
// card carries the bonus-site label that maps to the per-studio branding
// in stashdb.
//
// Supported entry URLs:
//
//	https://porngutter.com/                        → /updates/  (full catalogue)
//	https://porngutter.com/updates/                → same
//	https://porngutter.com/bonus_site/{slug}/      → per-bonus-site filtered listing
//	https://smutpuppet.com/                        → /updates/  (alt brand, same catalogue)
//	https://blackandbig.com/                       → /updates/  (sister domain)
//	…and the other 16 sister domains listed in matchRe.
//
// Card markup:
//
//	<div class="video-item grid-item">
//	  <div class="item-wrapper">
//	    <a href="/update/2780/?nats=…" class="item-thumb">
//	      <img id="thumb-2780" src="https://fast-media.roguebucks.com/.../cover.jpg" class="img" …>
//	      <video id="thumbvideo-2780" class="preview-video" …>
//	        <source src="https://fast-media.roguebucks.com/.../10_sec.mp4" type="video/mp4">
//	      </video>
//	    </a>
//	    <div class="item-content">
//	      <div class="item-cblock">
//	        <p><a href="/models/2072/" class="item-talent female">Elektra Rose</a></p>
//	        <a href="/update/2780/?nats=…">Black Cock Destroys Young Slut Elektra Rose</a>
//	      </div>
//	      <div class="item-cblock">
//	        <p>
//	          May 28, 2026 <br>
//	          <a href="/bonus_site/smut-merchants/?nats=…" class="quick-tag">Smut Merchants</a>
//	        </p>
//	      </div>
//	    </div>
//	  </div>
//	</div>
//
// Fields lifted from the card: ID, detail URL, title, performers, date
// (US format "January 2, 2006"), bonus-site label (stored on Scene.Series),
// thumbnail, preview video URL. Duration is not present on the card.
//
// Pagination: `?page=N` on both `/updates/` and `/bonus_site/{slug}/`. The
// `<ul class="pagination">` block contains a `class="page-link"` with the
// final page number, used to estimate the total scene count. Past-end
// pages return zero cards (clean stop signal). The default sort is
// newest-first so `KnownIDs` early-stop works.
//
// Detail pages route to `https://join.{site}/signup/signup.php?nats=…`
// (paywall) — listing-only, no detail fetch.
package porngutter

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const (
	defaultBase = "https://porngutter.com"
	scraperID   = "porngutter"
	studioName  = "Porn Gutter"
)

type Scraper struct {
	client *http.Client
}

func New() *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second)}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return scraperID }

func (s *Scraper) Patterns() []string {
	return []string{
		"porngutter.com/",
		"porngutter.com/updates/",
		"porngutter.com/bonus_site/{slug}/",
		"smutpuppet.com/",
		"3wayfuck.com/",
		"blackandbig.com/",
		"blondeslovedick.com/",
		"brunetteslovedick.com/",
		"darksodomy.com/",
		"dothewife.com/",
		"genlez.com/",
		"goldenslut.com/",
		"grannyvsbbc.com/",
		"groupfucksite.com/",
		"hcjav.com/",
		"maturefucksteen.com/",
		"milfsodomy.com/",
		"porn-uk.com/",
		"smutmerchants.com/",
		"suggabunny.com/",
		"teamfucksgirl.com/",
		"teenerotica.xxx/",
	}
}

// matchRe accepts every sister domain in the Porn Gutter network. Keep this
// list in sync with Patterns() — the registry test confirms a 1:1 match.
var matchRe = regexp.MustCompile(
	`^https?://(?:www\.|home2\.)?(?:` +
		`porngutter\.com|` +
		`smutpuppet\.com|` +
		`3wayfuck\.com|` +
		`blackandbig\.com|` +
		`blondeslovedick\.com|` +
		`brunetteslovedick\.com|` +
		`darksodomy\.com|` +
		`dothewife\.com|` +
		`genlez\.com|` +
		`goldenslut\.com|` +
		`grannyvsbbc\.com|` +
		`groupfucksite\.com|` +
		`hcjav\.com|` +
		`maturefucksteen\.com|` +
		`milfsodomy\.com|` +
		`porn-uk\.com|` +
		`smutmerchants\.com|` +
		`suggabunny\.com|` +
		`teamfucksgirl\.com|` +
		`teenerotica\.xxx` +
		`)(?:/|$)`,
)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// resolveListingPath normalises an input URL's path to the catalog path we
// should fetch.
//
//   - "/" / empty                    → "/updates/"
//   - "/updates" or "/updates/"      → "/updates/"
//   - "/bonus_site/{slug}/"          → kept as-is (filtered listing)
//   - "/update/{id}/…"               → rejected (detail-page URL, paywalled)
//
// All Porn Gutter sister domains serve the same global `/updates/` catalogue,
// so the host doesn't influence which scenes you get — only the path does.
func resolveListingPath(rawPath string) (string, error) {
	p := strings.TrimSpace(rawPath)
	if p == "" || p == "/" {
		return "/updates/", nil
	}
	if p == "/updates" || p == "/updates/" {
		return "/updates/", nil
	}
	if bonusSitePathRe.MatchString(p) {
		// Ensure trailing slash so query-string append is consistent.
		if !strings.HasSuffix(p, "/") {
			p += "/"
		}
		return p, nil
	}
	if detailPathRe.MatchString(p) {
		return "", fmt.Errorf("porngutter: %q is a paywalled detail page, not a listing", p)
	}
	return "", fmt.Errorf("porngutter: unsupported path %q (try / or /updates/ or /bonus_site/{slug}/)", p)
}

var (
	bonusSitePathRe = regexp.MustCompile(`^/bonus_site/[A-Za-z0-9][A-Za-z0-9-]*/?$`)
	detailPathRe    = regexp.MustCompile(`^/update/\d+/`)

	// Card parsing.
	cardStartRe  = regexp.MustCompile(`<div class="video-item grid-item">`)
	thumbIDRe    = regexp.MustCompile(`id="thumb-(\d+)"`)
	thumbImgRe   = regexp.MustCompile(`<img[^>]+id="thumb-\d+"[^>]+src="([^"]+)"`)
	previewSrcRe = regexp.MustCompile(`(?s)<video[^>]+class="preview-video"[^>]*>.*?<source[^>]+src="([^"]+)"`)
	// Title: the second /update/{id}/ anchor in the card. The first /update/
	// anchor wraps the item-thumb image (so its inner content starts with
	// `<img …>`); the title anchor wraps plain text. Constraining the capture
	// group to text-only (`[^<]+?`) is enough to skip the image wrapper —
	// Go's RE2 engine has no lookahead.
	titleAnchorRe = regexp.MustCompile(
		`(?s)<a\s+href="/update/\d+/[^"]*"[^>]*>\s*([^<][^<]*?)\s*</a>`,
	)
	// Performer anchor — `<a href="/models/N/" class="item-talent…">Name</a>`.
	performerRe = regexp.MustCompile(
		`<a[^>]+href="/models/\d+/?[^"]*"[^>]+class="item-talent[^"]*"[^>]*>\s*([^<]+?)\s*</a>`,
	)
	// Bonus-site label (the per-scene branding chip).
	bonusSiteAnchorRe = regexp.MustCompile(
		`<a[^>]+href="/bonus_site/([A-Za-z0-9-]+)/[^"]*"[^>]+class="[^"]*quick-tag[^"]*"[^>]*>\s*([^<]+?)\s*</a>`,
	)
	// Date "May 28, 2026" sitting in a free <p> block alongside the
	// quick-tag anchor.
	dateRe = regexp.MustCompile(
		`(January|February|March|April|May|June|July|August|September|October|November|December)\s+\d{1,2},\s+\d{4}`,
	)
	// Last page link: `<a class="page-link" href="?page=216&…">…</a>` — we
	// take the largest page number anywhere in the pagination block so a
	// missing `last-page` class on small catalogs doesn't matter.
	pageLinkRe = regexp.MustCompile(`\?page=(\d+)`)
)

type sceneItem struct {
	id         string
	title      string
	url        string // path-only
	thumb      string
	preview    string
	performers []string
	series     string
	date       time.Time
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
		if m := thumbIDRe.FindStringSubmatch(block); m != nil {
			item.id = m[1]
			item.url = "/update/" + m[1] + "/"
		}
		if item.id == "" || seen[item.id] {
			continue
		}
		seen[item.id] = true

		if m := titleAnchorRe.FindStringSubmatch(block); m != nil {
			item.title = html.UnescapeString(strings.TrimSpace(m[1]))
		}
		if m := thumbImgRe.FindStringSubmatch(block); m != nil {
			item.thumb = m[1]
		}
		if m := previewSrcRe.FindStringSubmatch(block); m != nil {
			item.preview = m[1]
		}
		for _, pm := range performerRe.FindAllStringSubmatch(block, -1) {
			name := html.UnescapeString(strings.TrimSpace(pm[1]))
			if name != "" {
				item.performers = append(item.performers, name)
			}
		}
		if m := bonusSiteAnchorRe.FindStringSubmatch(block); m != nil {
			item.series = html.UnescapeString(strings.TrimSpace(m[2]))
		}
		if m := dateRe.FindString(block); m != "" {
			if d, err := time.Parse("January 2, 2006", m); err == nil {
				item.date = d.UTC()
			}
		}

		items = append(items, item)
	}
	return items
}

// estimateTotal reads the largest `?page=N` value from the pagination block.
// Returns perPage when no pagination is present (single-page listing).
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

// listingURL builds the page-N URL preserving the host of the user's input
// (so smutpuppet.com input keeps smutpuppet.com host) and any query
// parameters already present (the `nats=` affiliate code, mostly — which we
// don't need but don't actively strip either).
func (s *Scraper) listingURL(entryURL, path string, page int) (string, error) {
	parsed, err := url.Parse(entryURL)
	if err != nil {
		return "", err
	}
	parsed.Path = path
	q := parsed.Query()
	q.Set("page", strconv.Itoa(page))
	parsed.RawQuery = q.Encode()
	return parsed.String(), nil
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	parsed, err := url.Parse(studioURL)
	if err != nil || parsed.Host == "" {
		select {
		case out <- scraper.Error(fmt.Errorf("porngutter: invalid URL %q: %w", studioURL, err)):
		case <-ctx.Done():
		}
		return
	}
	listingPath, err := resolveListingPath(parsed.Path)
	if err != nil {
		select {
		case out <- scraper.Error(err):
		case <-ctx.Done():
		}
		return
	}
	scraper.Debugf(1, "porngutter: scraping %s%s", parsed.Host, listingPath)

	now := time.Now().UTC()
	base := parsed.Scheme + "://" + parsed.Host

	scraper.Paginate(ctx, opts, "porngutter", out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL, err := s.listingURL(studioURL, listingPath, page)
		if err != nil {
			return scraper.PageResult{}, fmt.Errorf("porngutter: build URL: %w", err)
		}

		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}

		items := parseListing(body)
		scenes := make([]models.Scene, len(items))
		for i, item := range items {
			scenes[i] = item.toScene(base, studioURL, now)
		}
		return scraper.PageResult{
			Scenes: scenes,
			Total:  estimateTotal(body, len(items)),
		}, nil
	})
}

func (item sceneItem) toScene(siteBase, studioURL string, now time.Time) models.Scene {
	full := item.url
	if strings.HasPrefix(full, "/") {
		full = siteBase + full
	}
	return models.Scene{
		ID:         item.id,
		SiteID:     scraperID,
		StudioURL:  studioURL,
		Title:      item.title,
		URL:        full,
		Thumbnail:  item.thumb,
		Preview:    item.preview,
		Date:       item.date,
		Performers: item.performers,
		Studio:     studioName,
		Series:     item.series,
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
