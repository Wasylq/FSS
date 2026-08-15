// Package stripzvr scrapes stripzvr.com, a WordPress/Elementor VR site with no
// catalogue listing of any kind — `/videos/`, `/models/` and the other obvious
// paths all 404, and the home page links only a rotating handful of scenes.
//
// The page sitemaps are therefore the enumeration. Scene pages are exactly the
// two-segment paths `/{performer}/{scene}/`; everything else is either a
// one-segment site page or a `/members/{performer}/{scene}/` mirror of a scene
// already reachable publicly. The first segment is the performer slug, which is
// how the credit is recovered — the page itself never marks it up.
//
// Metadata comes from the Yoast JSON-LD `WebPage` node (title, description,
// publication date, thumbnail). The theme exposes no runtime, tags or
// categories anywhere, so those stay empty rather than being guessed at.
package stripzvr

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Anastylosis/FSS/internal/httpx"
	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/scraper"
)

const (
	siteID     = "stripzvr"
	domain     = "stripzvr.com"
	studioName = "StripzVR"
)

// sitemaps are the page sitemaps in index order. The site splits its pages
// across two rather than nesting them under one index the crawler can discover.
var sitemaps = []string{"/page-sitemap.xml", "/page-sitemap2.xml"}

// membersPrefix marks the paywalled mirror of a scene that is also published
// publicly. Scraping both would store every scene twice under two ids.
const membersPrefix = "members"

type Scraper struct {
	Client       *http.Client
	baseOverride string
}

func New() *Scraper {
	return &Scraper{Client: httpx.NewClient(30 * time.Second)}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

var (
	matchRe = regexp.MustCompile(`^https?://(?:www\.)?stripzvr\.com(?:/|$)`)
	locRe   = regexp.MustCompile(`<loc>\s*([^<\s]+)\s*</loc>`)
)

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		domain,
		domain + "/{performer}/{scene}/",
	}
}

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	if opts.Workers <= 0 {
		opts.Workers = 4
	}
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// base resolves the origin to fetch from. The operator's own host wins so a
// non-www spelling stays addressable; baseOverride is the test server. Sitemap
// entries are absolute live URLs, so they are re-based onto this rather than
// fetched verbatim — otherwise an offline test would still hit production.
func (s *Scraper) base(studioURL string) string {
	if s.baseOverride != "" {
		return strings.TrimSuffix(s.baseOverride, "/")
	}
	if u, err := url.Parse(studioURL); err == nil && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}
	return "https://www." + domain
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	base := s.base(studioURL)

	// A single scene URL is a legitimate thing to point at, and the sitemap
	// walk would be a strange way to honour it.
	if p, ok := scenePath(studioURL); ok {
		scraper.Debugf(1, "%s: scraping one scene %s", siteID, p)
		if !send(ctx, out, scraper.Progress(1)) {
			return
		}
		send(ctx, out, scraper.Scene(s.buildScene(ctx, studioURL, base, p, out)))
		return
	}

	paths, err := s.discover(ctx, base, opts)
	if err != nil {
		send(ctx, out, scraper.Error(err))
		return
	}
	if len(paths) == 0 {
		send(ctx, out, scraper.Error(scraper.ParseError(base+sitemaps[0],
			fmt.Errorf("no scene pages in the sitemaps"))))
		return
	}

	scraper.Debugf(1, "%s: %d scenes in the sitemaps, fetching with %d workers", siteID, len(paths), opts.Workers)
	if !send(ctx, out, scraper.Progress(len(paths))) {
		return
	}
	s.fetchAll(ctx, studioURL, base, paths, opts, out)
}

// discover reads the page sitemaps and returns the scene paths, de-duplicated
// and in sitemap order.
func (s *Scraper) discover(ctx context.Context, base string, opts scraper.ListOpts) ([]string, error) {
	seen := make(map[string]bool)
	var paths []string

	for i, sm := range sitemaps {
		if ctx.Err() != nil {
			return paths, ctx.Err()
		}
		if i > 0 && !sleep(ctx, opts.Delay) {
			return paths, ctx.Err()
		}
		scraper.Debugf(1, "%s: reading sitemap %s", siteID, sm)
		body, err := s.fetch(ctx, base+sm)
		if err != nil {
			// The second sitemap only exists while the catalogue is large
			// enough to need it. Missing the first one is fatal; missing a
			// later one is not worth failing the whole run over.
			if i == 0 {
				return nil, fmt.Errorf("sitemap %s: %w", sm, err)
			}
			scraper.Debugf(1, "%s: sitemap %s unavailable: %v", siteID, sm, err)
			continue
		}
		for _, m := range locRe.FindAllStringSubmatch(string(body), -1) {
			p, ok := scenePath(html.UnescapeString(m[1]))
			if !ok || seen[p] {
				continue
			}
			seen[p] = true
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// scenePath reports whether a URL names a scene page, and returns its path.
//
// Scene pages are exactly `/{performer}/{scene}/`. One segment is a site page
// (`/community/`, `/p8/`); three or more is the `/members/` mirror of a scene
// that is already public, which would otherwise be stored a second time under
// its own id.
func scenePath(rawURL string) (string, bool) {
	p := rawURL
	if u, err := url.Parse(rawURL); err == nil && u.Path != "" {
		p = u.Path
	}
	segs := strings.Split(strings.Trim(p, "/"), "/")
	if len(segs) != 2 || segs[0] == "" || segs[1] == "" {
		return "", false
	}
	if segs[0] == membersPrefix {
		return "", false
	}
	return "/" + segs[0] + "/" + segs[1] + "/", true
}

func (s *Scraper) fetchAll(ctx context.Context, studioURL, base string, paths []string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	work := make(chan string)
	var wg sync.WaitGroup
	// LIFO: close(work) ends the workers' range loops, then wg.Wait blocks
	// until they are gone, so a ctx.Done bail below cannot leak them.
	defer wg.Wait()
	defer close(work)

	for i := 0; i < opts.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range work {
				if !sleep(ctx, opts.Delay) {
					return
				}
				sc := s.buildScene(ctx, studioURL, base, p, out)
				if sc.ID == "" {
					continue
				}
				if !send(ctx, out, scraper.Scene(sc)) {
					return
				}
			}
		}()
	}

	for _, p := range paths {
		select {
		case work <- p:
		case <-ctx.Done():
			return
		}
	}
}

// buildScene fetches one scene page. A zero-value ID means the page could not
// be read and the failure has already been reported.
func (s *Scraper) buildScene(ctx context.Context, studioURL, base, path string, out chan<- scraper.SceneResult) models.Scene {
	sceneURL := base + path
	segs := strings.Split(strings.Trim(path, "/"), "/")

	body, err := s.fetch(ctx, sceneURL)
	if err != nil {
		send(ctx, out, scraper.Error(fmt.Errorf("scene %s: %w", path, err)))
		return models.Scene{}
	}

	page := parsePage(string(body))
	if page.title == "" {
		send(ctx, out, scraper.Error(scraper.ParseError(sceneURL, fmt.Errorf("no WebPage node on the scene page"))))
		return models.Scene{}
	}

	scene := models.Scene{
		// `{performer}/{scene}` — the theme exposes no numeric post id, and
		// the scene slug alone is not unique: the site reuses titles across
		// performers (`/alisia/hot-pants/` and `/arina-shy/hot-pants/`), which
		// collapsed 357 scenes to 306 when the slug was the id on its own.
		ID:          segs[0] + "/" + segs[1],
		SiteID:      siteID,
		StudioURL:   studioURL,
		Title:       page.title,
		URL:         sceneURL,
		Studio:      studioName,
		Description: page.description,
		Thumbnail:   page.thumbnail,
		Date:        page.date,
		Performers:  []string{performerName(page.title, segs[0])},
		ScrapedAt:   time.Now().UTC(),
	}
	return scene
}

func (s *Scraper) fetch(ctx context.Context, u string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		Method:  http.MethodGet,
		URL:     u,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}

// ---- page parsing ----

type pageData struct {
	title       string
	description string
	thumbnail   string
	date        time.Time
}

var ldJSONRe = regexp.MustCompile(`(?s)<script type="application/ld\+json"[^>]*>(.*?)</script>`)

// yoastGraph is the subset of Yoast's `@graph` this reads. Only the WebPage
// node matters; the WebSite node repeats `name` and `description` for the whole
// site and would otherwise overwrite the scene's own.
type yoastGraph struct {
	Graph []struct {
		Type          string `json:"@type"`
		Name          string `json:"name"`
		Description   string `json:"description"`
		ThumbnailURL  string `json:"thumbnailUrl"`
		DatePublished string `json:"datePublished"`
	} `json:"@graph"`
}

func parsePage(body string) pageData {
	var d pageData
	for _, m := range ldJSONRe.FindAllStringSubmatch(body, -1) {
		var g yoastGraph
		if err := json.Unmarshal([]byte(m[1]), &g); err != nil {
			continue
		}
		for _, node := range g.Graph {
			if node.Type != "WebPage" {
				continue
			}
			d.title = cleanTitle(node.Name)
			d.description = cleanText(node.Description)
			d.thumbnail = node.ThumbnailURL
			if t, err := time.Parse(time.RFC3339, node.DatePublished); err == nil {
				d.date = t.UTC()
			}
		}
	}
	if d.title == "" {
		d.title = cleanTitle(firstSubmatch(ogRe("title"), body))
	}
	if d.description == "" {
		d.description = cleanText(firstSubmatch(ogRe("description"), body))
	}
	if d.thumbnail == "" {
		d.thumbnail = firstSubmatch(ogRe("image"), body)
	}
	return d
}

func ogRe(prop string) *regexp.Regexp {
	return regexp.MustCompile(`<meta property="og:` + prop + `" content="([^"]*)"`)
}

// titleSuffixRe strips the site branding Yoast appends to every page title:
// `Cum Back For You featuring Tiny Tina @ StripzVR.com - StripzVR`.
var titleSuffixRe = regexp.MustCompile(`(?i)\s*(?:featuring\s+.*)?@\s*StripzVR\.com.*$`)

func cleanTitle(s string) string {
	return cleanText(titleSuffixRe.ReplaceAllString(s, ""))
}

// featuringRe recovers the credit from the page title, which is the only place
// the site spells the performer's name properly. The URL slug is the fallback.
var featuringRe = regexp.MustCompile(`(?i)featuring\s+([^@|\-]+?)\s*(?:@|\||-|$)`)

func performerName(title, slug string) string {
	if m := featuringRe.FindStringSubmatch(title); m != nil {
		if n := cleanText(m[1]); n != "" {
			return n
		}
	}
	return slugToName(slug)
}

// slugToName turns `melena-maria-rya` into `Melena Maria Rya`. It is a
// last resort: capitalisation the site itself uses is always preferred.
func slugToName(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

// ---- helpers ----

var tagStripRe = regexp.MustCompile(`<[^>]*>`)

func cleanText(s string) string {
	s = tagStripRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, " ", " ")
	return strings.Join(strings.Fields(s), " ")
}

func firstSubmatch(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	select {
	case <-time.After(d):
		return true
	case <-ctx.Done():
		return false
	}
}

func send(ctx context.Context, out chan<- scraper.SceneResult, r scraper.SceneResult) bool {
	select {
	case out <- r:
		return true
	case <-ctx.Done():
		return false
	}
}
