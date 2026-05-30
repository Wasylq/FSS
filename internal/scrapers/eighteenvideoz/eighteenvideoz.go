// Package eighteenvideoz scrapes 18videoz.com and its Wow-Tube
// (SeriousCash) sister sites. Each site registers its own scraper ID, but
// they all share one parser that handles the four card-format variants
// the network has emitted over the years:
//
//   - "VariantA-parent": the parent portal at 18videoz.com — outer
//     `<div class="th">` with `re_add_click_old('{id}', '{site_id}')`,
//     `<span class="caption">` title, `<span class="author">` source
//     sub-site label, `<span class="time">MM:SS</span>`.
//   - "VariantA-child": same family used by teeny-lovers-style children —
//     `<div id="{id}" class="th">` with `re_add_click('{id}', '{site_id}')`
//     (no `_old` suffix), title still in `<span class="caption">`. No
//     `<span class="author">` since the card is already site-scoped.
//   - "VariantB-rich": the modern child layout —
//     `<div class="th" onclick="add_click('{id}', …)">` with hidden
//     `<div id="desc{id}">description</div>` + `<div id="time{id}">00:00 / MM:SS</div>`
//     and the user-facing title in `<p class="d1">Title</p>`. Used by
//     casualteensex, teensanalyzed, sellyourgf, youngsexparties, teenylovers
//     (a few of these also publish a detail-page anchor we keep).
//   - "VariantC-thumb": younglibertines uses `<div class="thumb">` (note
//     singular) with `show_trailer('{id}'); add_click('{id}', '{site_id}')`,
//     a `<p>` description inside `<div class="desc">` (no separate title),
//     and `<div class="time">MM:SS</div>`.
//   - "VariantD-table": the legacy static-HTML layout used by older
//     sister sites (firstanaldate, bangmyteenass, olddicksyoungchix).
//     Each scene is a multi-row `<table>` with `<td class="title">Title</td>`,
//     an image grid where filenames `images/{id}-{n}.jpg` encode the scene
//     ID, and optionally a `<td class="description">Description</td>`.
//     Pagination via `index{N}.htm` links. No detail pages — scene anchor
//     synthesised as `{base}/#scene-{id}` like the parent.
//
// Package name: Go identifiers cannot start with a digit, so the package
// is spelled out as `eighteenvideoz`. The user-facing scraper ID for the
// parent is `18videoz`.
//
// Pagination paths used across sites:
//
//   - `/index.php/main/show_sets2/{N}` — parent (Variant A-parent)
//   - `/index.php/main/show_sets/{N}`  — Variant A-child + Variant C
//   - `/index{N}.htm`                  — Variant D (no leading dot — `index2.htm`,
//     `index3.htm`, etc.)
//   - ""                               — single-page (homepage IS the
//     full catalogue, used by teensanalyzed, teenylovers); in that case
//     the scraper fetches `/` once and stops.
//
// In every case the listing is newest-first, so `KnownIDs` early-stop
// works without specifying a sort parameter. Detail pages are not public
// on the parent (every onclick goes to the sub-site's signup) — we
// synthesise `{base}/#scene-{id}` URLs. The child sites with a real
// `/index.php/main/show_sets/{id}/0` detail URL get that lifted into
// `Scene.URL` instead.
//
// Out-of-scope sister domains (would each need its own custom scraper /
// not currently working):
//
//   - 18flesh.com, homepornotapes.com, ishootmygirl.com, mydirtygf.com,
//     olddicksyoungchix.com, pornfilms3d.com — dead (empty / 0-byte
//     responses).
//   - bangmyteenass.com — pure promo landing, no scene cards.
//   - firstanaldate.com, nasty-angels.com, girlsnextdoorabused.com —
//     alive but use distinct layouts that don't match any of the three
//     variants here.
package eighteenvideoz

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

const studioName = "18videoz"

// SiteConfig describes one Wow-Tube sister site. SiteBase has no trailing
// slash. SiteName is the human-readable label stored on Scene.Series —
// empty for the parent portal, since each card on the parent carries its
// own per-scene `<span class="author">` label that overrides this default.
type SiteConfig struct {
	ID       string
	SiteBase string
	SiteName string
	// PaginationPath is the URL path used for pagination, with `{N}`
	// substituted at runtime. Empty means single-page (homepage IS the
	// full catalogue; the scraper fetches `/` once and stops).
	PaginationPath string
	Patterns       []string
	MatchRe        *regexp.Regexp
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

// Card parsing.
//
// We use a permissive card-start anchor — any `<div class="th">` or
// `<div class="thumb">` (variant C) — and within each card we try every
// known field-extraction regex, picking whichever matches. That keeps the
// parser robust across the three card variants without a per-site
// dispatch.
var (
	// Card start. Matches:
	//   - <div class="th">             (variants A-parent, A-child, B)
	//   - <div id="N" class="th">      (variant A-child)
	//   - <div class="thumb">          (variant C, younglibertines)
	//   - <td ...class="title">        (variant D — legacy table layout)
	// Variant C: singular `thumb`. We anchor with `>` after the class
	// to avoid matching `<div class="thumbs">` (the wrapping container).
	cardStartRe = regexp.MustCompile(
		`<div(?: id="\d+")? class="(?:th|thumb)"[^>]*>` +
			`|<td[^>]*\bclass="title"[^>]*>`,
	)

	// Scene ID — try every onclick handler form in turn:
	//   - re_add_click_old('123', '1')  (parent)
	//   - re_add_click('123', '1')      (variant A-child)
	//   - show_trailer('123');add_click('123', …)  (variant B, C)
	//   - id="thumb_123"                (variant B fallback)
	//   - id="123" class="th"           (variant A-child fallback)
	clickIDRe  = regexp.MustCompile(`(?:re_add_click(?:_old)?|add_click|show_trailer)\(['"](\d+)['"]`)
	idThumbRe  = regexp.MustCompile(`id=["']?thumb_(\d+)`)
	idAttrThRe = regexp.MustCompile(`<div id="(\d+)" class="th"`)

	// Title:
	//   - <span class="caption">Title</span>            (A-parent, A-child)
	//   - <p class="d1" …>Title</p>                     (variant B)
	//   - <div id="title{id}" …>Title</div>             (variant B, hidden)
	captionRe  = regexp.MustCompile(`(?s)<span class="caption">\s*([^<]+?)\s*</span>`)
	d1Re       = regexp.MustCompile(`(?s)<p\s+class="d1"[^>]*>\s*([^<]+?)\s*</p>`)
	titleDivRe = regexp.MustCompile(`(?s)<div id="title(\d+)"[^>]*>\s*([^<]+?)\s*</div>`)

	// Source sub-site (parent only) — <span class="author">…</span>.
	authorRe = regexp.MustCompile(`(?s)<span class="author">\s*([^<]+?)\s*</span>`)

	// Description (variant B/C):
	//   - <div id="desc{id}" …>Long description</div>   (variant B)
	//   - <div class="desc">…<p …>Description</p>…      (variant C — first <p>)
	descDivRe = regexp.MustCompile(`(?s)<div id="desc(\d+)"[^>]*>\s*([^<]+?)\s*</div>`)
	descPRe   = regexp.MustCompile(`(?s)<div class="desc"[^>]*>.*?<p[^>]*>\s*([^<]+?)\s*[<]`)

	// Duration:
	//   - <span class="time">MM:SS</span>               (A-parent)
	//   - <div class="time" …>MM:SS</div>               (variant C)
	//   - <div id="time{id}" …>HH:MM / MM:SS</div>      (variant B — second number is total)
	timeSpanRe = regexp.MustCompile(`<span class="time">\s*(\d{1,2}):(\d{2})\s*</span>`)
	timeDivRe  = regexp.MustCompile(`<div class="time"[^>]*>\s*(\d{1,2}):(\d{2})\s*</div>`)
	timeIDRe   = regexp.MustCompile(`<div id="time\d+"[^>]*>\s*\d+:\d+\s*/\s*(\d{1,2}):(\d{2})\s*</div>`)

	// Thumbnail — first `<img>` inside the card pointing at the site's
	// CDN. We tolerate either `cdn2.{site}` or any host under `*.{site}`.
	thumbRe = regexp.MustCompile(`<img[^>]+src="(https?://[^"]+/(?:images|tour/images|pictures)/[^"]+\.(?:jpg|jpeg|png|webp))"`)

	// Detail URL (variant B sites that expose one) — `/index.php/main/show_sets/{id}/0`.
	detailURLRe = regexp.MustCompile(`href="(/index\.php/main/show_sets/(\d+)/\d+)"`)

	// Variant D — legacy table layout.
	//   - Title is INSIDE the matched `<td ... class="title">`, captured
	//     directly as the cell's text content.
	//   - ID is encoded in the next image filename `images/{id}-{n}.jpg`
	//     that follows the title cell, where {id} is typically 1-4 digits
	//     and {n} is a 1-digit photo index. We accept any extension.
	//   - Description (when present) is in a sibling
	//     `<td class="description">…</td>` cell.
	tdTitleRe = regexp.MustCompile(
		`(?s)<td[^>]*\bclass="title"[^>]*>\s*([^<]+?)\s*</td>`,
	)
	variantDImgIDRe = regexp.MustCompile(
		`<img[^>]+src="(?:[^"]*/)?images/(\d{1,5})-\d+\.(?:jpg|jpeg|png|webp)"`,
	)
	tdDescriptionRe = regexp.MustCompile(
		`(?s)<td[^>]*\bclass="description"[^>]*>\s*([^<]+?)\s*</td>`,
	)
	// Variant D pagination — `<a href="indexN.htm">` (no leading slash on
	// some sites, with on others; either form is matched).
	indexHTMRe = regexp.MustCompile(`href="[^"]*\bindex(\d+)\.html?"`)

	// Pagination max-page lookup — any /show_sets2/{N} or /show_sets/{N}
	// href in the page. We take the highest.
	pageLinkRe = regexp.MustCompile(`href="[^"]*/show_sets2?/(\d+)"`)
)

type sceneItem struct {
	id          string
	title       string
	description string
	series      string // per-scene source sub-site label (parent only)
	duration    int    // seconds
	thumb       string
	detailPath  string // path-only e.g. "/index.php/main/show_sets/174/0"
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
		// Variant D fast-path: the card start was a `<td class="title">`
		// — the title is captured by the same regex, and the ID is in the
		// next images/{id}-{n}.{ext} filename.
		if m := tdTitleRe.FindStringSubmatch(block); m != nil {
			item.title = html.UnescapeString(strings.TrimSpace(m[1]))
			if im := variantDImgIDRe.FindStringSubmatch(block); im != nil {
				item.id = im[1]
				// Thumbnail: the image filename itself, anchored to the
				// site's base in toScene.
				if tm := variantDImgIDRe.FindStringSubmatchIndex(block); tm != nil {
					// Pull the full match (group 0) for the URL.
					thumb := variantDImgIDRe.FindString(block)
					if i := strings.Index(thumb, `src="`); i >= 0 {
						thumb = thumb[i+5:]
						if j := strings.Index(thumb, `"`); j >= 0 {
							item.thumb = thumb[:j]
						}
					}
				}
			}
			if dm := tdDescriptionRe.FindStringSubmatch(block); dm != nil {
				item.description = html.UnescapeString(strings.TrimSpace(dm[1]))
			}
			// Variant D has no title field to synthesise — title is already
			// the td content. Fall through to the dedup check.
			if item.id == "" || seen[item.id] {
				continue
			}
			seen[item.id] = true
			items = append(items, item)
			continue
		}

		// ID — try the onclick handler, then the id="thumb_N" form, then
		// the id="N" attribute on the outer div itself.
		if m := clickIDRe.FindStringSubmatch(block); m != nil {
			item.id = m[1]
		}
		if item.id == "" {
			if m := idThumbRe.FindStringSubmatch(block); m != nil {
				item.id = m[1]
			}
		}
		if item.id == "" {
			if m := idAttrThRe.FindStringSubmatch(block); m != nil {
				item.id = m[1]
			}
		}
		if item.id == "" || seen[item.id] {
			continue
		}
		seen[item.id] = true

		// Title — try variants in order: hidden title div (variant B —
		// most authoritative), then caption (variant A), then d1 (variant
		// B fallback).
		for _, m := range titleDivRe.FindAllStringSubmatch(block, -1) {
			if m[1] == item.id {
				item.title = html.UnescapeString(strings.TrimSpace(m[2]))
				break
			}
		}
		if item.title == "" {
			if m := captionRe.FindStringSubmatch(block); m != nil {
				item.title = html.UnescapeString(strings.TrimSpace(m[1]))
			}
		}
		if item.title == "" {
			if m := d1Re.FindStringSubmatch(block); m != nil {
				item.title = html.UnescapeString(strings.TrimSpace(m[1]))
			}
		}

		// Description — hidden id="descN", then desc-p fallback.
		for _, m := range descDivRe.FindAllStringSubmatch(block, -1) {
			if m[1] == item.id {
				item.description = html.UnescapeString(strings.TrimSpace(m[2]))
				break
			}
		}
		if item.description == "" {
			if m := descPRe.FindStringSubmatch(block); m != nil {
				item.description = html.UnescapeString(strings.TrimSpace(m[1]))
			}
		}

		// Variant C cards (younglibertines) have no title field — only a
		// description. Synthesise a title from the first sentence so Stash
		// has something to display.
		if item.title == "" && item.description != "" {
			item.title = synthesizeTitle(item.description, item.id)
		}

		// Source sub-site label (parent only — children leave this empty
		// and SiteName is used downstream).
		if m := authorRe.FindStringSubmatch(block); m != nil {
			item.series = html.UnescapeString(strings.TrimSpace(m[1]))
		}

		// Duration — try the three forms in turn.
		for _, re := range []*regexp.Regexp{timeSpanRe, timeDivRe, timeIDRe} {
			if m := re.FindStringSubmatch(block); m != nil {
				mins, _ := strconv.Atoi(m[1])
				secs, _ := strconv.Atoi(m[2])
				item.duration = mins*60 + secs
				break
			}
		}

		// Thumbnail.
		if m := thumbRe.FindStringSubmatch(block); m != nil {
			item.thumb = m[1]
		}

		// Detail URL (only on sites that expose one).
		if m := detailURLRe.FindStringSubmatch(block); m != nil {
			item.detailPath = m[1]
		}

		items = append(items, item)
	}
	return items
}

func estimateTotal(body []byte, perPage int) int {
	maxPage := 1
	// Variants A/B/C use /show_sets[2]/{N}.
	for _, m := range pageLinkRe.FindAllSubmatch(body, -1) {
		n, _ := strconv.Atoi(string(m[1]))
		if n > maxPage {
			maxPage = n
		}
	}
	// Variant D uses index{N}.htm.
	for _, m := range indexHTMRe.FindAllSubmatch(body, -1) {
		n, _ := strconv.Atoi(string(m[1]))
		if n > maxPage {
			maxPage = n
		}
	}
	return maxPage * perPage
}

// listingURL builds the URL for page N.
//
//   - PaginationPath empty            → "/" (single-page mode)
//   - PaginationPath contains "{N}"   → substitute N (Variant D, e.g.
//     `/index{N}.htm`); page 1 strips the page suffix and returns the
//     homepage.
//   - Otherwise                       → "{base}{path}/{N}" (Variants A/B/C)
func (s *Scraper) listingURL(page int) string {
	if s.cfg.PaginationPath == "" {
		return s.cfg.SiteBase + "/"
	}
	if strings.Contains(s.cfg.PaginationPath, "{N}") {
		// Variant D — page 1 is the bare homepage (no `index1.htm` link
		// exists for it, the root path serves that view).
		if page <= 1 {
			return s.cfg.SiteBase + "/"
		}
		return s.cfg.SiteBase + strings.ReplaceAll(s.cfg.PaginationPath, "{N}", strconv.Itoa(page))
	}
	return fmt.Sprintf("%s%s/%d", s.cfg.SiteBase, s.cfg.PaginationPath, page)
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)
	scraper.Debugf(1, "%s: scraping", s.cfg.ID)

	now := time.Now().UTC()
	singlePage := s.cfg.PaginationPath == ""
	scraper.Paginate(ctx, opts, s.cfg.ID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := s.listingURL(page)
		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}

		items := parseListing(body)
		if len(items) == 0 {
			return scraper.PageResult{}, nil
		}

		total := estimateTotal(body, len(items))
		scenes := make([]models.Scene, len(items))
		for i, item := range items {
			scenes[i] = s.toScene(item, studioURL, now)
		}
		return scraper.PageResult{Scenes: scenes, Total: total, Done: singlePage}, nil
	})
}

func (s *Scraper) toScene(item sceneItem, studioURL string, now time.Time) models.Scene {
	// Per-scene series label takes priority on the parent (where each
	// card carries `<span class="author">`); otherwise fall back to the
	// site name.
	series := item.series
	if series == "" {
		series = s.cfg.SiteName
	}

	// URL: prefer a real detail-page path if the site exposed one,
	// otherwise synthesise an anchor.
	url := s.cfg.SiteBase + "/#scene-" + item.id
	if item.detailPath != "" {
		url = s.cfg.SiteBase + item.detailPath
	}

	// Variant D thumbs are root-relative (`images/092-1.jpg`) since the
	// legacy static layout doesn't prefix them. Anchor to the site base so
	// downstream consumers get an absolute URL.
	thumb := item.thumb
	if thumb != "" && !strings.HasPrefix(thumb, "http") {
		thumb = s.cfg.SiteBase + "/" + strings.TrimLeft(thumb, "/")
	}

	return models.Scene{
		ID:          item.id,
		SiteID:      s.cfg.ID,
		StudioURL:   studioURL,
		Title:       item.title,
		Description: item.description,
		URL:         url,
		Thumbnail:   thumb,
		Duration:    item.duration,
		Studio:      studioName,
		Series:      series,
		ScrapedAt:   now,
	}
}

// synthesizeTitle pulls the first sentence (or first 80 chars) out of a
// description to act as a Scene title. Falls back to "Scene {id}" when the
// description is empty.
func synthesizeTitle(desc, id string) string {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return "Scene " + id
	}
	for _, sep := range []string{". ", "! ", "? "} {
		if i := strings.Index(desc, sep); i > 0 {
			return strings.TrimSpace(desc[:i+1])
		}
	}
	const maxLen = 80
	if len(desc) <= maxLen {
		return desc
	}
	trim := desc[:maxLen]
	if cut := strings.LastIndex(trim, " "); cut > 40 {
		trim = trim[:cut]
	}
	return strings.TrimSpace(trim) + "…"
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
