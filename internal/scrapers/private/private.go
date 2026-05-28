// Package private scrapes Private (private.com) and the sister sites in
// its tree that share the main private.com CMS. The 9 sister "site"
// brands that only ship a landing page (analintroductions.com,
// blacksonsluts.com, etc.) are auto-rewritten to their canonical
// `https://www.private.com/site/{slug}/` listing path.
//
// Out-of-scope (different CMSes, would each need their own package):
//
//   - privateblack.com    (different layout, own catalogue)
//   - privatecastings.com (different layout, own catalogue)
//   - privateclassics.com (different layout, own catalogue)
//   - trannytemptation.com (empty)
//
// Supported entry URLs:
//
//	https://www.private.com/                        → /scenes/         (full catalogue)
//	https://www.private.com/scenes  /scenes/{N}     → main listing, page-N
//	https://www.private.com/movies  /movies/{N}     → movies (DVD-based) listing
//	https://www.private.com/site/{slug}/  …/{N}     → per-sub-site filtered
//	https://www.private.com/pornstar/{id}-{slug}/   → per-pornstar
//	https://analintroductions.com/                  → /site/anal-introductions/
//	…and 8 other landing-only sister domains.
//
// Card markup (one card):
//
//	<li class="card">
//	  <div class="scene">
//	    <a data-track="SCENE_LINK"
//	       href="https://www.private.com/scene/{slug}/{id}" …>
//	      <picture> … <img src="https://pcom77.st-content.com/…/506037.jpg" …> </picture>
//	      <span class="overThumb mobile_trailer" …>
//	        <video class="mini_video_player" …>
//	          <source src="https://pcoms77.st-content.com/…/trailer_02.mp4" …>
//	        </video>
//	      </span>
//	    </a>
//	    <ul class="scene-details">
//	      <li class="ultrahdlabel"><a><span>4K</span></a></li>
//	    </ul>
//	    <div class="desc-scene">
//	      <h3>
//	        <a data-track="TITLE_LINK" href="…/scene/{slug}/{id}">Title</a>
//	      </h3>
//	      <ul class="scene-models">
//	        <li><a data-track="PORNSTAR_LINK" href="/pornstar/{id}-{slug}/">Name</a></li>
//	      </ul>
//	      <span class="scene-date">04/04/2026</span>
//	    </div>
//	  </div>
//	</li>
//
// Fields lifted from the card: ID (from /scene/.../{id}), URL, title,
// performers (multi), date (MM/DD/YYYY), thumbnail, preview MP4. Tags,
// duration, and the long description live on the detail page but we keep
// the listing-only path for speed — `/scenes/` already paginates to ~50
// pages of metadata-rich cards.
//
// Pagination: path-based `/scenes/{N}` and `/site/{slug}/{N}`. The
// `<ul class="pagination">` block always contains a "Skip to page …"
// helper with the highest page number, used for total estimation. Past-end
// pages return zero cards (clean stop signal).
//
// Sort: the default listing is newest-first (the homepage section is
// literally `data-track-action="LATEST_SCENES"`), so `KnownIDs` early-stop
// works.
package private

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
	canonicalBase = "https://www.private.com"
	scraperID     = "private"
	studioName    = "Private"
)

type Scraper struct {
	client *http.Client
}

func New() *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second)}
}

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return scraperID }

func (s *Scraper) Patterns() []string {
	return []string{
		"private.com/",
		"private.com/scenes",
		"private.com/movies",
		"private.com/site/{slug}/",
		"private.com/pornstar/{id}-{slug}/",
		"analintroductions.com/",
		"blacksonsluts.com/",
		"iconfessfiles.com/",
		"missionasspossible.com/",
		"privatefetish.com/",
		"privatemilfs.com/",
		"russianfakeagent.com/",
		"russianteenass.com/",
		"tightandteen.com/",
	}
}

// hostRewrite maps the landing-only sister-domain hosts to their canonical
// /site/{slug}/ path on private.com. Sister domains with their own
// catalogue and different CMS (privateblack.com, privatecastings.com,
// privateclassics.com, trannytemptation.com) are intentionally absent.
var hostRewrite = map[string]string{
	"analintroductions.com":  "/site/anal-introductions/",
	"blacksonsluts.com":      "/site/blacks-on-sluts/",
	"iconfessfiles.com":      "/site/i-confess-files/",
	"missionasspossible.com": "/site/mission-ass-possible/",
	"privatefetish.com":      "/site/private-fetish/",
	"privatemilfs.com":       "/site/private-milfs/",
	"russianfakeagent.com":   "/site/russian-fake-agent/",
	"russianteenass.com":     "/site/russian-teen-ass/",
	"tightandteen.com":       "/site/tight-and-teen/",
}

var matchRe = regexp.MustCompile(
	`^https?://(?:www\.)?(?:` +
		`private\.com|` +
		`analintroductions\.com|` +
		`blacksonsluts\.com|` +
		`iconfessfiles\.com|` +
		`missionasspossible\.com|` +
		`privatefetish\.com|` +
		`privatemilfs\.com|` +
		`russianfakeagent\.com|` +
		`russianteenass\.com|` +
		`tightandteen\.com` +
		`)(?:/|$)`,
)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// resolved holds the canonical fetch target derived from the user's input.
//
//   - base is always "https://www.private.com" — even when the user fed a
//     sister domain, the actual content sits on private.com.
//   - basePath is the listing path with no trailing page number. We append
//     /{N} for N > 1.
//   - basePath ends with "/" when it's a directory-style path
//     (`/site/foo/`, `/pornstar/X-bar/`) and without it for `/scenes`/
//     `/movies` (pagination is `/scenes/2`, not `/scenes//2`).
type resolved struct {
	base     string
	basePath string
}

// resolveInput normalises the user-provided URL to a (base, basePath) pair.
//
// Path rules:
//
//   - "/" or empty            → "/scenes"
//   - "/scenes" or "/scenes/" → "/scenes"
//   - "/scenes/{N}"           → "/scenes" (page will be re-derived from the loop)
//   - "/movies" similar       → "/movies"
//   - "/site/{slug}/" or "/site/{slug}/{N}" → "/site/{slug}/"
//   - "/pornstar/{id}-{slug}/" → "/pornstar/{id}-{slug}/"
//   - "/scene/{slug}/{id}"    → error (detail page, not a listing)
//
// Sister-domain hosts in `hostRewrite` redirect to the corresponding
// /site/{slug}/ path on private.com.
func resolveInput(rawURL string) (resolved, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return resolved{}, fmt.Errorf("private: invalid URL %q", rawURL)
	}
	host := strings.ToLower(parsed.Host)
	host = strings.TrimPrefix(host, "www.")

	// Sister-domain rewrite — host carries the meaning, ignore the path.
	if path, ok := hostRewrite[host]; ok {
		return resolved{base: canonicalBase, basePath: path}, nil
	}

	if host != "private.com" {
		return resolved{}, fmt.Errorf("private: unsupported host %q", parsed.Host)
	}

	p := strings.TrimSpace(parsed.Path)
	if p == "" || p == "/" {
		return resolved{base: canonicalBase, basePath: "/scenes"}, nil
	}
	// Detail-page guard.
	if detailPathRe.MatchString(p) {
		return resolved{}, fmt.Errorf("private: %q is a scene detail page, not a listing", p)
	}
	if m := scenesPathRe.FindStringSubmatch(p); m != nil {
		return resolved{base: canonicalBase, basePath: "/scenes"}, nil
	}
	if m := moviesPathRe.FindStringSubmatch(p); m != nil {
		return resolved{base: canonicalBase, basePath: "/movies"}, nil
	}
	if m := sitePathRe.FindStringSubmatch(p); m != nil {
		return resolved{base: canonicalBase, basePath: "/site/" + m[1] + "/"}, nil
	}
	if m := pornstarPathRe.FindStringSubmatch(p); m != nil {
		return resolved{base: canonicalBase, basePath: "/pornstar/" + m[1] + "/"}, nil
	}
	return resolved{}, fmt.Errorf("private: unsupported path %q", p)
}

var (
	// /scene/{slug}/{id} — detail page, must NOT be a listing input.
	detailPathRe = regexp.MustCompile(`^/scene/[^/]+/\d+/?$`)
	// /scenes or /scenes/{N}
	scenesPathRe = regexp.MustCompile(`^/scenes/?(?:\d+/?)?$`)
	// /movies or /movies/{N}
	moviesPathRe = regexp.MustCompile(`^/movies/?(?:\d+/?)?$`)
	// /site/{slug}/ or /site/{slug}/{N}/
	sitePathRe = regexp.MustCompile(`^/site/([A-Za-z0-9][A-Za-z0-9-]*)/?(?:\d+/?)?$`)
	// /pornstar/{id}-{slug}/ or /pornstar/{id}-{slug}/{N}/
	pornstarPathRe = regexp.MustCompile(`^/pornstar/(\d+-[A-Za-z0-9-]+)/?(?:\d+/?)?$`)

	// Card parsing.
	cardStartRe = regexp.MustCompile(`<li class="card">`)
	// Scene URL on private.com — the numeric ID at the end is the stable
	// scene identifier we use everywhere.
	sceneURLRe = regexp.MustCompile(
		`href="(https?://(?:www\.)?private\.com/scene/[^"]+/(\d+))(?:\?[^"]*)?"`,
	)
	// Title anchor — `<a data-track="TITLE_LINK" …>Title</a>`. Constraining
	// the capture group to `[^<]+` skips the thumbnail-wrapping anchor.
	titleAnchorRe = regexp.MustCompile(
		`(?s)<a[^>]+data-track="TITLE_LINK"[^>]*>\s*([^<]+?)\s*</a>`,
	)
	// Performer anchor — `<a data-track="PORNSTAR_LINK" …>Name</a>`.
	performerRe = regexp.MustCompile(
		`<a[^>]+data-track="PORNSTAR_LINK"[^>]*>\s*([^<]+?)\s*</a>`,
	)
	// Date span — `<span class="scene-date">MM/DD/YYYY</span>`.
	dateRe = regexp.MustCompile(`<span class="scene-date">\s*(\d{2}/\d{2}/\d{4})\s*</span>`)
	// Thumbnail — `<img … src="...contentthumbs/N.jpg" …>` inside the card.
	// We grab the eager-load src attribute, not the srcset.
	thumbRe = regexp.MustCompile(`<img[^>]+src="(https?://[^"]*/contentthumbs/[^"]+\.jpg)"`)
	// Preview MP4 — `<source src="...trailers/...mp4" …>`.
	previewRe = regexp.MustCompile(`<source[^>]+src="(https?://[^"]+/trailers/[^"]+\.mp4)`)
	// Pagination — `<a href="https://www.private.com/scenes/{N}">…</a>` or
	// `<a href="/scenes/{N}">…</a>`. We pull the largest page number anywhere
	// in the page (covers both the visible pages and the "Skip to page"
	// helper that lists 50, 100, etc.).
	paginationPageRe = regexp.MustCompile(`href="[^"]*/(?:scenes|movies|site/[A-Za-z0-9-]+|pornstar/\d+-[A-Za-z0-9-]+)/(\d+)/?"`)
)

type sceneItem struct {
	id         string
	url        string
	title      string
	performers []string
	date       time.Time
	thumb      string
	preview    string
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

		if m := titleAnchorRe.FindStringSubmatch(block); m != nil {
			item.title = html.UnescapeString(strings.TrimSpace(m[1]))
		}
		for _, pm := range performerRe.FindAllStringSubmatch(block, -1) {
			name := html.UnescapeString(strings.TrimSpace(pm[1]))
			if name != "" {
				item.performers = append(item.performers, name)
			}
		}
		item.performers = dedupStrings(item.performers)
		if m := dateRe.FindStringSubmatch(block); m != nil {
			if d, err := time.Parse("01/02/2006", m[1]); err == nil {
				item.date = d.UTC()
			}
		}
		if m := thumbRe.FindStringSubmatch(block); m != nil {
			item.thumb = m[1]
		}
		if m := previewRe.FindStringSubmatch(block); m != nil {
			item.preview = m[1]
		}

		items = append(items, item)
	}
	return items
}

// estimateTotal returns the largest /{N}/ page-number value found in any
// listing-style pagination link, multiplied by perPage. Defaults to
// perPage when no pagination is present.
func estimateTotal(body []byte, perPage int) int {
	maxPage := 1
	for _, m := range paginationPageRe.FindAllSubmatch(body, -1) {
		n, _ := strconv.Atoi(string(m[1]))
		if n > maxPage {
			maxPage = n
		}
	}
	return maxPage * perPage
}

// listingURL builds the page-N URL for a resolved listing target. Page 1
// is the bare basePath; pages > 1 append /{N} (with the trailing slash if
// basePath ends in one, no slash otherwise — private.com is strict about
// it for /scenes/2 vs /site/foo/2/).
func listingURL(r resolved, page int) string {
	if page <= 1 {
		return r.base + r.basePath
	}
	switch {
	case strings.HasSuffix(r.basePath, "/"):
		return r.base + r.basePath + strconv.Itoa(page) + "/"
	default:
		return r.base + r.basePath + "/" + strconv.Itoa(page)
	}
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	target, err := resolveInput(studioURL)
	if err != nil {
		defer close(out)
		select {
		case out <- scraper.Error(err):
		case <-ctx.Done():
		}
		return
	}
	s.runFromResolved(ctx, target, studioURL, opts, out)
}

// runFromResolved is the inner pagination loop, split out so tests can
// drive it against a httptest server URL without going through
// resolveInput (which would reject hosts other than private.com /
// sister-domain). Closes `out` on return.
func (s *Scraper) runFromResolved(ctx context.Context, target resolved, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)
	scraper.Debugf(1, "private: scraping %s%s", target.base, target.basePath)

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

		pageURL := listingURL(target, page)
		scraper.Debugf(1, "private: fetching page %d (%s)", page, pageURL)

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
			return
		}

		if !sentTotal {
			total := estimateTotal(body, len(items))
			scraper.Debugf(1, "private: ~%d total scenes (estimated)", total)
			if total > 0 {
				select {
				case out <- scraper.Progress(total):
				case <-ctx.Done():
					return
				}
			}
			sentTotal = true
		}

		series := seriesFromPath(target.basePath)
		for _, item := range items {
			if opts.KnownIDs[item.id] {
				scraper.Debugf(1, "private: hit known ID %s, stopping early", item.id)
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
				return
			}
			select {
			case out <- scraper.Scene(item.toScene(studioURL, series, now)):
			case <-ctx.Done():
				return
			}
		}
	}
}

// seriesFromPath returns the human-friendly Series label for a per-sub-site
// listing path. "/site/anal-introductions/" → "Anal Introductions". For
// non-site listings (`/scenes`, `/movies`, `/pornstar/...`) it returns ""
// so Series stays empty.
func seriesFromPath(p string) string {
	const prefix = "/site/"
	if !strings.HasPrefix(p, prefix) {
		return ""
	}
	slug := strings.Trim(p[len(prefix):], "/")
	if slug == "" {
		return ""
	}
	// Convert "anal-introductions" → "Anal Introductions". Each hyphenated
	// token is title-cased; preserve any token shorter than two characters
	// untouched to avoid mangling things like "i-confess-files" → "I
	// Confess Files".
	parts := strings.Split(slug, "-")
	for i, w := range parts {
		if w == "" {
			continue
		}
		parts[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(parts, " ")
}

func (item sceneItem) toScene(studioURL, series string, now time.Time) models.Scene {
	return models.Scene{
		ID:         item.id,
		SiteID:     scraperID,
		StudioURL:  studioURL,
		Title:      item.title,
		URL:        item.url,
		Thumbnail:  item.thumb,
		Preview:    item.preview,
		Date:       item.date,
		Performers: item.performers,
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
