// Package dirtyflix scrapes the Dirty Flix all-access portal and the
// SeriousCash sister-brand domains in its tree. The network's sister
// sites each ship their own custom theme variant, so the package
// dispatches to one of four parsers via a per-site `Variant` field:
//
//   - VariantThumbsItem: the parent portal dirtyflix.com. Card markup is
//     `<div class="thumbs-item">` + `re_add_click_old('{id}')` onclick,
//     title in `<a class="title">`, source sub-brand in
//     `<a class="link">{Brand}</a>` which is lifted onto Scene.Series so
//     each scene gets tagged with its true brand. Pagination is
//     single-page (the same ~80 scenes regardless of page number).
//
//   - VariantBrutalX: brutalx.com. Card is `<div id="{id}" class="th"
//     onclick="click_me('{id}')">` with title in
//     `<h3 class="title_thumb">`, duration in
//     `<span class="duration"><em>MM:SS</em></span>`, resolution in
//     `<span class="size">`. Pagination via
//     `/index.php/main/show_sets2/{N}` (80 cards/page).
//
//   - VariantThumbWrap: kinkyfamily.com, x-sensual.com,
//     privatecasting-x.com. Card is `<a class="thumb_wrap"
//     onclick="re_add_click('{id}', …)">` with title in either
//     `<span class="caption">Title</span>` (kinkyfamily,
//     privatecasting-x) or `<span class="caption"><h3>Title</h3></span>`
//     (x-sensual) — the parser tries both. Duration in either
//     `<span class="item_box"><em>MM:SS</em></span>` or
//     `<span class="duration">MM:SS</span>`. Pagination via
//     `/index.php/main/show_sets2/{N}`.
//
// In every variant detail pages are paywalled — scene URLs are
// synthesised as `{base}/#scene-{id}` like the 18videoz / Porn Gutter
// parents. Sister sites not yet covered (each needs its own theme
// parser): debtsex, disgracethatbitch, fuckingglasses (similar to
// BrutalX but title in `<span class="desc">`), makehimcuckold /
// sheisnerdy / momspassions / trickyourgf (`<div class="movie-block">`
// theme with thumbnail-path-encoded IDs), massage-x / spypov, trickyagent,
// youngcourtesans. Marked as future work in partially-covered tracking.
package dirtyflix

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

const studioName = "Dirty Flix"

// Variant selects which parser + pagination URL form to use for a site.
type Variant int

const (
	VariantThumbsItem Variant = iota
	VariantBrutalX
	VariantThumbWrap
	// VariantFuckingGlasses — `<a class="th" onclick="re_add_click_old">` +
	// `<span class="desc">Title</span>` + `<span class="item"><i class="icon-clock"/><span>MM:SS</span></span>`.
	VariantFuckingGlasses
	// VariantMovieBlock — the make-him-cuckold family. `<div class="movie-block">`
	// outer; scene ID encoded in thumbnail URL `tour_thumbs/{prefix}{N}/`,
	// title in `<a class="link">`, duration in `<span class="time">MM:SS</span>`,
	// description in `<div class="description">`.
	VariantMovieBlock
	// VariantYoungCourtesans — `<div class="thumb" id="thN">` + `re_add_click('id', …)`
	// + `<span class="thumb-title">` + `<span class="thumb-time">MM:SS</span>`.
	VariantYoungCourtesans
	// VariantDebtsex — `<div class="thumb thumb1">` + `re_add_click('id', …)`
	// + `<div class="thumb-desc--title"><a>Title</a></div>` + `<i>MM:SS</i>`
	// + `<a class="model-from">Performer</a>` + `<span>YYYY-MM-DD</span>`.
	VariantDebtsex
	// VariantDisgrace — `<div id="N" class="thumb" onclick="add_click('N')">`
	// + `<a class="th-link">Title</a>`. No duration in card.
	VariantDisgrace
)

// SiteConfig describes one Dirty Flix network site.
type SiteConfig struct {
	ID       string
	SiteBase string // no trailing slash
	SiteName string // human-readable label → Scene.Series default
	Variant  Variant
	// Paginated is true for sister sites that walk pages; false for the
	// parent (single-page, fetch homepage once).
	Paginated bool
	Patterns  []string
	MatchRe   *regexp.Regexp
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

// ---- VariantThumbsItem (parent portal) ----

var (
	thumbsItemCardStartRe = regexp.MustCompile(`<div class="thumbs-item"\s*>`)
	thumbsItemClickIDRe   = regexp.MustCompile(`re_add_click_old\(\s*['"](\d+)['"]`)
	thumbsItemTitleRe     = regexp.MustCompile(`<a[^>]+class="title"[^>]*>\s*([^<]+?)\s*</a>`)
	thumbsItemSiteLinkRe  = regexp.MustCompile(`<a[^>]+class="link"[^>]*>\s*([^<]+?)\s*</a>`)
	thumbsItemDurationRe  = regexp.MustCompile(`<span class="duration[^"]*">\s*(\d{1,2}):(\d{2})\s*</span>`)
	thumbsItemResRe       = regexp.MustCompile(`<span class="resolution[^"]*">\s*([^<]+?)\s*</span>`)
	thumbsItemThumbRe     = regexp.MustCompile(`<img class="im0"[^>]+src="([^"]+)"`)
)

// ---- VariantBrutalX ----

var (
	// Card start: `<div id="N"` + class containing "th" (matches both
	// brutalx's `class="th"` and spypov's `class="th thumb_item"`).
	brutalXCardStartRe = regexp.MustCompile(`<div id="(\d+)"[^>]+class="th[^"]*"[^>]*>`)
	// Title: `<h3 class="title_thumb">` (brutalx, massage-x) OR
	// `<span class="title_thumb">` (spypov).
	brutalXTitleRe = regexp.MustCompile(`<(?:h3|span) class="title_thumb"[^>]*>\s*([^<]+?)\s*</(?:h3|span)>`)
	// Duration is in `<span class="duration"><em>MM:SS</em></span>`.
	brutalXDurationRe = regexp.MustCompile(`<span class="duration"[^>]*>\s*<em>\s*(\d{1,2}):(\d{2})\s*</em>`)
	brutalXResRe      = regexp.MustCompile(`<span class="size">\s*([^<]+?)\s*</span>`)
	// Thumb is the `<img>` inside `<span class="thumb_img">`. Brutalx has
	// a spacer img first; massage-x/spypov go straight to the real thumb.
	// The `class="thumb-img"` variant covers brutalx; the bare-src variant
	// covers massage-x/spypov.
	brutalXThumbClassRe = regexp.MustCompile(`<img[^>]+src="([^"]+)"[^>]+class="thumb-img"`)
	brutalXThumbImgRe   = regexp.MustCompile(`<span class="thumb_img"[^>]*>\s*<img[^>]+src="([^"]+)"`)
)

// ---- VariantThumbWrap ----

var (
	thumbWrapCardStartRe = regexp.MustCompile(`<a\s+class="thumb_wrap"[^>]*>`)
	thumbWrapClickIDRe   = regexp.MustCompile(`re_add_click\(\s*['"](\d+)['"]`)
	// Title comes in two flavours: a bare text caption (kinkyfamily,
	// privatecasting-x) or a caption-wrapped <h3> (x-sensual). The
	// caption-h3 form is tried first because the h3 capture is more
	// specific; the bare-caption regex below excludes any captures that
	// start with a `<` so it won't false-match an h3-wrapping caption.
	//
	// `(?s)` lets `.` match newlines so we can skip arbitrary nested
	// content (like a `<span class="duration">…</span>` sibling) between
	// the caption opener and the h3.
	thumbWrapTitleH3Re   = regexp.MustCompile(`(?s)<span class="caption"[^>]*>.*?<h3[^>]*>\s*([^<]+?)\s*</h3>`)
	thumbWrapTitleTextRe = regexp.MustCompile(`<span class="caption"[^>]*>\s*([^<][^<]*?)\s*</span>`)
	// Duration: either bare `<span class="duration">MM:SS</span>` (x-sensual)
	// or `<span class="item_box"><em>MM:SS</em></span>` (kinkyfamily).
	thumbWrapDurationSpanRe = regexp.MustCompile(`<span class="duration">\s*(\d{1,2}):(\d{2})\s*</span>`)
	thumbWrapDurationEmRe   = regexp.MustCompile(`<span class="item_box"[^>]*><i[^>]+icon-clock[^>]*></i>\s*<em>\s*(\d{1,2}):(\d{2})\s*</em>`)
	// Thumb — first `<img>` inside `<span class="wrap_image">` or
	// `<span class="thumbnail-img">`.
	thumbWrapImgRe = regexp.MustCompile(`<span class="(?:wrap_image|thumbnail-img)"[^>]*>\s*<img[^>]+src="([^"]+)"`)
	// We then skip past the spacer placeholder (.../spacer.gif or spacer2/3.gif).
	thumbWrapImg2Re = regexp.MustCompile(`<img[^>]+src="([^"]+)"[^>]+class="thumb-img"`)
	// Resolution badge — `<span class="hd">4k</span>` or
	// `<span class="k4">4k</span>`. Capture is text-only, so nested
	// markup variants (`<span class="quality"><span>4k</span></span>`)
	// safely no-match here — that's fine, resolution is best-effort.
	thumbWrapResRe = regexp.MustCompile(`<span class="(?:hd|k4)">\s*([^<]+?)\s*</span>`)
)

// ---- VariantFuckingGlasses ----

var (
	fgCardStartRe = regexp.MustCompile(`<a class="th"\s+onclick="re_add_click_old\(\s*['"]?(\d+)['"]?\s*\)"`)
	fgTitleRe     = regexp.MustCompile(`<span class="desc">\s*([^<]+?)\s*</span>`)
	// Duration: `<i class="icon-clock"></i> <span>23:40</span>` inside
	// `<span class="item">`.
	fgDurationRe = regexp.MustCompile(`(?s)<i class="icon-clock"[^>]*></i>\s*<span>\s*(\d{1,2}):(\d{2})\s*</span>`)
	fgThumbRe    = regexp.MustCompile(`<span class="wrap_image">\s*<img[^>]+src="([^"]+)"`)
	// Quality: `<span class="quality"><span>4k</span></span>` — capture
	// the inner span's text.
	fgQualityRe = regexp.MustCompile(`<span class="quality">\s*<span>\s*([^<]+?)\s*</span>`)
)

// ---- VariantMovieBlock (makehimcuckold cluster) ----

var (
	// Card start: bare `<div class="movie-block">` (covers both the
	// older makehimcuckold/trickyourgf form with no extra attributes and
	// the newer sheisnerdy/momspassions/trickyagent form with
	// `id="movie_N" data-movie="N"`).
	movieBlockCardStartRe = regexp.MustCompile(`<div class="movie-block"[^>]*>`)
	// ID — preferred form is `data-movie="N"` (newer sites); fall back to
	// the CDN folder slug `tour_thumbs/{prefix}{N}/` for makehimcuckold
	// and trickyourgf where no data-movie attribute is emitted.
	movieBlockDataMovieRe  = regexp.MustCompile(`data-movie="(\d+)"`)
	movieBlockIDFallbackRe = regexp.MustCompile(`/tour_thumbs/([a-z]+\d+)/`)
	// Title — `<a class="…link[^"]*">Title</a>` (makehimcuckold,
	// sheisnerdy, momspassions, trickyourgf). Fallback: `<h3>Title</h3>`
	// inside `<div class="title">` (trickyagent uses that form).
	movieBlockTitleRe    = regexp.MustCompile(`<a[^>]+class="[^"]*\blink\b[^"]*"[^>]*>\s*([^<]+?)\s*</a>`)
	movieBlockTitleH3Re  = regexp.MustCompile(`(?s)<div class="title[^"]*">\s*<h3[^>]*>\s*([^<]+?)\s*</h3>`)
	movieBlockDurationRe = regexp.MustCompile(`<span class="time">\s*(\d{1,2}):(\d{2})\s*</span>`)
	movieBlockDescRe     = regexp.MustCompile(`(?s)<div class="description">\s*([^<]+?)\s*</div>`)
	movieBlockThumbRe    = regexp.MustCompile(`<img\s+src="(https?://[^"]*/tour_thumbs/[^"]+\.(?:jpg|jpeg|png|webp))"`)
)

// ---- VariantYoungCourtesans ----

var (
	ycCardStartRe = regexp.MustCompile(`<div class="thumb" id="th\d+"\s*>`)
	ycClickIDRe   = regexp.MustCompile(`re_add_click\(\s*['"](\d+)['"]`)
	ycTitleRe     = regexp.MustCompile(`<span class="thumb-title">\s*([^<]+?)\s*</span>`)
	ycDurationRe  = regexp.MustCompile(`<span class="thumb-time">\s*(\d{1,2}):(\d{2})\s*</span>`)
	ycQualityRe   = regexp.MustCompile(`<span class="quality">\s*([^<]+?)\s*</span>`)
	ycThumbRe     = regexp.MustCompile(`<img[^>]+src="([^"]+)"[^>]+class="thumb-img"`)
)

// ---- VariantDebtsex ----

var (
	debtsexCardStartRe = regexp.MustCompile(`<div class="thumb thumb\d+" id="th\d+"`)
	debtsexClickIDRe   = regexp.MustCompile(`re_add_click\(\s*['"](\d+)['"]`)
	debtsexTitleRe     = regexp.MustCompile(`(?s)<div class="thumb-desc--title"><a[^>]*>\s*([^<]+?)\s*</a></div>`)
	// Duration is bare `<i>34:46</i>` inside the thumb-img anchor.
	debtsexDurationRe  = regexp.MustCompile(`<i>\s*(\d{1,2}):(\d{2})\s*</i>`)
	debtsexPerformerRe = regexp.MustCompile(`<a[^>]+class="model-from"[^>]*>\s*([^<]+?)\s*</a>`)
	debtsexDateRe      = regexp.MustCompile(`<span>\s*(\d{4}-\d{2}-\d{2})\s*</span>`)
	debtsexThumbRe     = regexp.MustCompile(`<div class="thumb-img">\s*<a[^>]*>\s*<img[^>]+src="([^"]+)"`)
)

// ---- VariantDisgrace ----

var (
	disgraceCardStartRe = regexp.MustCompile(`<div\s+id="(\d+)"\s+class="thumb"\s+onclick="add_click\('?\d+`)
	disgraceTitleRe     = regexp.MustCompile(`<a[^>]+class="th-link"[^>]*>\s*([^<]+?)\s*</a>`)
	disgraceThumbRe     = regexp.MustCompile(`<div class="t">\s*<img[^>]+src="([^"]+)"`)
)

// Pagination — any show_sets2/{N} or show_sets/{N} href.
var pageLinkRe = regexp.MustCompile(`href="[^"]*/show_sets2?/(\d+)"`)

type sceneItem struct {
	id         string
	title      string
	series     string
	duration   int // seconds
	thumb      string
	resolution string
}

func parseListing(body []byte, v Variant) []sceneItem {
	switch v {
	case VariantThumbsItem:
		return parseThumbsItem(body)
	case VariantBrutalX:
		return parseBrutalX(body)
	case VariantThumbWrap:
		return parseThumbWrap(body)
	case VariantFuckingGlasses:
		return parseFuckingGlasses(body)
	case VariantMovieBlock:
		return parseMovieBlock(body)
	case VariantYoungCourtesans:
		return parseYoungCourtesans(body)
	case VariantDebtsex:
		return parseDebtsex(body)
	case VariantDisgrace:
		return parseDisgrace(body)
	}
	return nil
}

func parseThumbsItem(body []byte) []sceneItem {
	page := string(body)
	starts := thumbsItemCardStartRe.FindAllStringIndex(page, -1)
	items := make([]sceneItem, 0, len(starts))
	seen := make(map[string]bool, len(starts))

	for i, loc := range starts {
		end := len(page)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		block := page[loc[0]:end]

		var item sceneItem
		if m := thumbsItemClickIDRe.FindStringSubmatch(block); m != nil {
			item.id = m[1]
		}
		if item.id == "" || seen[item.id] {
			continue
		}
		seen[item.id] = true

		if m := thumbsItemTitleRe.FindStringSubmatch(block); m != nil {
			item.title = html.UnescapeString(strings.TrimSpace(m[1]))
		}
		if m := thumbsItemSiteLinkRe.FindStringSubmatch(block); m != nil {
			item.series = html.UnescapeString(strings.TrimSpace(m[1]))
		}
		if m := thumbsItemDurationRe.FindStringSubmatch(block); m != nil {
			mins, _ := strconv.Atoi(m[1])
			secs, _ := strconv.Atoi(m[2])
			item.duration = mins*60 + secs
		}
		if m := thumbsItemResRe.FindStringSubmatch(block); m != nil {
			item.resolution = strings.TrimSpace(m[1])
		}
		if m := thumbsItemThumbRe.FindStringSubmatch(block); m != nil {
			item.thumb = m[1]
		}
		items = append(items, item)
	}
	return items
}

func parseBrutalX(body []byte) []sceneItem {
	page := string(body)
	starts := brutalXCardStartRe.FindAllStringSubmatchIndex(page, -1)
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
		if m := brutalXTitleRe.FindStringSubmatch(block); m != nil {
			item.title = html.UnescapeString(strings.TrimSpace(m[1]))
		}
		if m := brutalXDurationRe.FindStringSubmatch(block); m != nil {
			mins, _ := strconv.Atoi(m[1])
			secs, _ := strconv.Atoi(m[2])
			item.duration = mins*60 + secs
		}
		if m := brutalXResRe.FindStringSubmatch(block); m != nil {
			item.resolution = strings.TrimSpace(m[1])
		}
		if m := brutalXThumbClassRe.FindStringSubmatch(block); m != nil {
			item.thumb = m[1]
		} else if m := brutalXThumbImgRe.FindStringSubmatch(block); m != nil {
			item.thumb = m[1]
		}
		items = append(items, item)
	}
	return items
}

func parseThumbWrap(body []byte) []sceneItem {
	page := string(body)
	starts := thumbWrapCardStartRe.FindAllStringIndex(page, -1)
	items := make([]sceneItem, 0, len(starts))
	seen := make(map[string]bool, len(starts))

	for i, loc := range starts {
		end := len(page)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		block := page[loc[0]:end]

		var item sceneItem
		if m := thumbWrapClickIDRe.FindStringSubmatch(block); m != nil {
			item.id = m[1]
		}
		if item.id == "" || seen[item.id] {
			continue
		}
		seen[item.id] = true

		// Title — try h3 form first (x-sensual), then bare caption text.
		if m := thumbWrapTitleH3Re.FindStringSubmatch(block); m != nil {
			item.title = html.UnescapeString(strings.TrimSpace(m[1]))
		}
		if item.title == "" {
			if m := thumbWrapTitleTextRe.FindStringSubmatch(block); m != nil {
				item.title = html.UnescapeString(strings.TrimSpace(m[1]))
			}
		}
		// Duration — try both forms.
		if m := thumbWrapDurationSpanRe.FindStringSubmatch(block); m != nil {
			mins, _ := strconv.Atoi(m[1])
			secs, _ := strconv.Atoi(m[2])
			item.duration = mins*60 + secs
		}
		if item.duration == 0 {
			if m := thumbWrapDurationEmRe.FindStringSubmatch(block); m != nil {
				mins, _ := strconv.Atoi(m[1])
				secs, _ := strconv.Atoi(m[2])
				item.duration = mins*60 + secs
			}
		}
		// Thumb — prefer the wrap_image / thumbnail-img inner img, but
		// the spacer is usually first. Use the class="thumb-img" fallback.
		if m := thumbWrapImg2Re.FindStringSubmatch(block); m != nil {
			item.thumb = m[1]
		} else if m := thumbWrapImgRe.FindStringSubmatch(block); m != nil {
			item.thumb = m[1]
		}
		// The spacer is universally `/images/spacer*.gif` or
		// `/pictures/spacer.gif`. If we accidentally captured one, look
		// for a second img.
		if isSpacer(item.thumb) {
			imgs := regexp.MustCompile(`<img[^>]+src="([^"]+)"`).FindAllStringSubmatch(block, -1)
			for _, im := range imgs {
				if !isSpacer(im[1]) {
					item.thumb = im[1]
					break
				}
			}
		}
		if m := thumbWrapResRe.FindStringSubmatch(block); m != nil {
			item.resolution = strings.TrimSpace(m[1])
		}
		items = append(items, item)
	}
	return items
}

func isSpacer(u string) bool {
	return strings.Contains(u, "/spacer") || strings.HasSuffix(u, "/spacer.gif")
}

func parseFuckingGlasses(body []byte) []sceneItem {
	page := string(body)
	starts := fgCardStartRe.FindAllStringSubmatchIndex(page, -1)
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
		if m := fgTitleRe.FindStringSubmatch(block); m != nil {
			item.title = html.UnescapeString(strings.TrimSpace(m[1]))
		}
		if m := fgDurationRe.FindStringSubmatch(block); m != nil {
			mins, _ := strconv.Atoi(m[1])
			secs, _ := strconv.Atoi(m[2])
			item.duration = mins*60 + secs
		}
		if m := fgThumbRe.FindStringSubmatch(block); m != nil {
			item.thumb = m[1]
		}
		if m := fgQualityRe.FindStringSubmatch(block); m != nil {
			item.resolution = strings.TrimSpace(m[1])
		}
		items = append(items, item)
	}
	return items
}

func parseMovieBlock(body []byte) []sceneItem {
	page := string(body)
	starts := movieBlockCardStartRe.FindAllStringIndex(page, -1)
	items := make([]sceneItem, 0, len(starts))
	seen := make(map[string]bool, len(starts))

	for i, loc := range starts {
		end := len(page)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		block := page[loc[0]:end]

		var id string
		// Try data-movie first; fall back to the CDN folder slug.
		if m := movieBlockDataMovieRe.FindStringSubmatch(block); m != nil {
			id = m[1]
		}
		if id == "" {
			if m := movieBlockIDFallbackRe.FindStringSubmatch(block); m != nil {
				id = m[1]
			}
		}
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		item := sceneItem{id: id}

		if m := movieBlockTitleRe.FindStringSubmatch(block); m != nil {
			item.title = html.UnescapeString(strings.TrimSpace(m[1]))
		}
		if item.title == "" {
			if m := movieBlockTitleH3Re.FindStringSubmatch(block); m != nil {
				item.title = html.UnescapeString(strings.TrimSpace(m[1]))
			}
		}
		if m := movieBlockDurationRe.FindStringSubmatch(block); m != nil {
			mins, _ := strconv.Atoi(m[1])
			secs, _ := strconv.Atoi(m[2])
			item.duration = mins*60 + secs
		}
		if m := movieBlockThumbRe.FindStringSubmatch(block); m != nil {
			item.thumb = m[1]
		}
		// Description is captured but not currently stored on sceneItem;
		// the description regex match is here so future expansion is
		// trivial.
		_ = movieBlockDescRe
		items = append(items, item)
	}
	return items
}

func parseYoungCourtesans(body []byte) []sceneItem {
	page := string(body)
	starts := ycCardStartRe.FindAllStringIndex(page, -1)
	items := make([]sceneItem, 0, len(starts))
	seen := make(map[string]bool, len(starts))

	for i, loc := range starts {
		end := len(page)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		block := page[loc[0]:end]

		var item sceneItem
		if m := ycClickIDRe.FindStringSubmatch(block); m != nil {
			item.id = m[1]
		}
		if item.id == "" || seen[item.id] {
			continue
		}
		seen[item.id] = true

		if m := ycTitleRe.FindStringSubmatch(block); m != nil {
			item.title = html.UnescapeString(strings.TrimSpace(m[1]))
		}
		if m := ycDurationRe.FindStringSubmatch(block); m != nil {
			mins, _ := strconv.Atoi(m[1])
			secs, _ := strconv.Atoi(m[2])
			item.duration = mins*60 + secs
		}
		if m := ycQualityRe.FindStringSubmatch(block); m != nil {
			item.resolution = strings.TrimSpace(m[1])
		}
		if m := ycThumbRe.FindStringSubmatch(block); m != nil {
			item.thumb = m[1]
		}
		items = append(items, item)
	}
	return items
}

func parseDebtsex(body []byte) []sceneItem {
	page := string(body)
	starts := debtsexCardStartRe.FindAllStringIndex(page, -1)
	items := make([]sceneItem, 0, len(starts))
	seen := make(map[string]bool, len(starts))

	for i, loc := range starts {
		end := len(page)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		block := page[loc[0]:end]

		var item sceneItem
		if m := debtsexClickIDRe.FindStringSubmatch(block); m != nil {
			item.id = m[1]
		}
		if item.id == "" || seen[item.id] {
			continue
		}
		seen[item.id] = true

		if m := debtsexTitleRe.FindStringSubmatch(block); m != nil {
			item.title = html.UnescapeString(strings.TrimSpace(m[1]))
		}
		if m := debtsexDurationRe.FindStringSubmatch(block); m != nil {
			mins, _ := strconv.Atoi(m[1])
			secs, _ := strconv.Atoi(m[2])
			item.duration = mins*60 + secs
		}
		if m := debtsexThumbRe.FindStringSubmatch(block); m != nil {
			item.thumb = m[1]
		}
		// Performer + date matches are captured below but not stored —
		// the sceneItem struct intentionally stays narrow for this CMS
		// family. Reserved for future expansion.
		_ = debtsexPerformerRe
		_ = debtsexDateRe
		items = append(items, item)
	}
	return items
}

func parseDisgrace(body []byte) []sceneItem {
	page := string(body)
	starts := disgraceCardStartRe.FindAllStringSubmatchIndex(page, -1)
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
		if m := disgraceTitleRe.FindStringSubmatch(block); m != nil {
			item.title = html.UnescapeString(strings.TrimSpace(m[1]))
		}
		if m := disgraceThumbRe.FindStringSubmatch(block); m != nil {
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
	if !s.cfg.Paginated || page <= 1 {
		return s.cfg.SiteBase + "/"
	}
	return fmt.Sprintf("%s/index.php/main/show_sets2/%d", s.cfg.SiteBase, page)
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)
	scraper.Debugf(1, "dirtyflix/%s: scraping (variant=%d)", s.cfg.ID, s.cfg.Variant)

	siteID := "dirtyflix/" + s.cfg.ID
	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := s.listingURL(page)
		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}

		items := parseListing(body, s.cfg.Variant)
		if len(items) == 0 {
			return scraper.PageResult{}, nil
		}

		var total int
		if s.cfg.Paginated {
			total = estimateTotal(body, len(items))
		} else {
			total = len(items)
		}

		scenes := make([]models.Scene, len(items))
		for i, item := range items {
			scenes[i] = s.toScene(item, studioURL, now)
		}
		return scraper.PageResult{Scenes: scenes, Total: total, Done: !s.cfg.Paginated}, nil
	})
}

func (s *Scraper) toScene(item sceneItem, studioURL string, now time.Time) models.Scene {
	series := item.series
	if series == "" {
		series = s.cfg.SiteName
	}
	return models.Scene{
		ID:         item.id,
		SiteID:     s.cfg.ID,
		StudioURL:  studioURL,
		Title:      item.title,
		URL:        fmt.Sprintf("%s/#scene-%s", s.cfg.SiteBase, item.id),
		Thumbnail:  item.thumb,
		Duration:   item.duration,
		Resolution: item.resolution,
		Studio:     studioName,
		Series:     series,
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
