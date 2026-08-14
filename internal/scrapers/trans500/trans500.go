// Package trans500 scrapes the Trans500 network tour (trans500.com/tour3).
//
// The network's StashDB children are categories of one catalogue rather than
// separate sites: ikillitts.com and tsgirlfriendexperience.com both redirect
// to bigbootytgirls.com, transatplay.com redirects to trans500.com, and
// superramon.com is a parked domain. What survives is `category.php?id=N`,
// whose ids line up with the children — 44 I Kill It TS, 46 Trans at Play,
// 47 TS Girlfriend Experience, 48 Super Ramon, 52 Behind Trans500 — and id 5,
// which is the whole catalogue and a strict superset of the rest (847 works
// against 289 for the largest child).
//
// So id 5 is what a bare trans500.com scrapes, and a `category.php?id=N` URL
// scrapes that child alone. Which brand a scene belongs to is not lost either
// way: the detail page states it as `Site: <name>`, which becomes Series.
//
// Big Booty TGirls is the one child with a live catalogue of its own; it runs
// a different template and lives in the bigbootytgirls package.
package trans500

import (
	"context"
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
	siteID     = "trans500"
	domain     = "trans500.com"
	studioName = "Trans500"
	// catalogueCategory is `category.php?id=5`, the unfiltered listing. Every
	// other category is a subset of it.
	catalogueCategory = "5"
)

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
	matchRe     = regexp.MustCompile(`^https?://(?:www\.)?trans500\.com(?:/|$)`)
	categoryRe  = regexp.MustCompile(`[?&]id=(\d+)`)
	modelPathRe = regexp.MustCompile(`/models/([^/?#]+)\.html`)
)

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		domain,
		domain + "/tour3/category.php?id={id}",
		domain + "/tour3/models/{slug}.html",
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

// base resolves the origin to fetch from. The operator's own host wins so an
// http-only or non-www spelling stays addressable; baseOverride is the test
// server. Every request is built as base + path, never from the absolute URLs
// the pages embed, so pointing base at httptest redirects the whole crawl.
func (s *Scraper) base(studioURL string) string {
	if s.baseOverride != "" {
		return strings.TrimSuffix(s.baseOverride, "/")
	}
	if u, err := url.Parse(studioURL); err == nil && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}
	return "https://" + domain
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	base := s.base(studioURL)

	if m := modelPathRe.FindStringSubmatch(studioURL); m != nil && m[1] != "models" {
		scraper.Debugf(1, "%s: scraping model %q", siteID, m[1])
		s.collect(ctx, studioURL, base, modelPages(m[1]), opts, out)
		return
	}

	id := catalogueCategory
	if m := categoryRe.FindStringSubmatch(studioURL); m != nil {
		id = m[1]
	}
	if id == catalogueCategory {
		scraper.Debugf(1, "%s: scraping the full catalogue", siteID)
	} else {
		scraper.Debugf(1, "%s: scraping category %s", siteID, id)
	}
	s.collect(ctx, studioURL, base, categoryPages(id), opts, out)
}

func categoryPages(id string) func(page int) string {
	return func(page int) string {
		return fmt.Sprintf("/tour3/category.php?id=%s&page=%d&s=d", id, page)
	}
}

// modelPages returns a single-page walk: a model page lists that performer's
// scenes in one go and has no pagination controls.
func modelPages(slug string) func(page int) string {
	return func(page int) string {
		if page > 1 {
			return ""
		}
		return "/tour3/models/" + slug + ".html"
	}
}

// maxListingPages bounds the walk. The catalogue is 28 pages today; the cap is
// a runaway guard, not a limit anyone should reach.
const maxListingPages = 400

// collect walks a paginated listing, then fetches each scene's detail page for
// the description and the brand it was published under.
func (s *Scraper) collect(ctx context.Context, studioURL, base string, pagePath func(int) string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	seen := make(map[string]bool)
	var entries []listEntry

	for page := 1; page <= maxListingPages; page++ {
		if ctx.Err() != nil {
			return
		}
		path := pagePath(page)
		if path == "" {
			break
		}
		if page > 1 && !sleep(ctx, opts.Delay) {
			return
		}

		scraper.Debugf(1, "%s: fetching listing %s", siteID, path)
		body, err := s.fetch(ctx, base+path)
		if err != nil {
			send(ctx, out, scraper.Error(fmt.Errorf("listing page %d: %w", page, err)))
			return
		}

		cards := parseCards(string(body))
		if len(cards) == 0 {
			if page == 1 {
				// A first page that fetched cleanly and parsed to nothing is a
				// template change, not an empty catalogue. Saying so keeps the
				// traversal non-authoritative rather than letting a --full run
				// read the silence as a catalogue that vanished.
				send(ctx, out, scraper.Error(scraper.ParseError(base+path, fmt.Errorf("no scene cards on the first listing page"))))
			}
			break
		}

		added := 0
		for _, c := range cards {
			if seen[c.id] {
				continue
			}
			seen[c.id] = true
			added++
			entries = append(entries, c)
		}
		// Past the last page the CMS re-serves the last one rather than 404ing,
		// so a page that adds nothing new is the end.
		if added == 0 {
			break
		}
	}

	if len(entries) == 0 {
		return
	}

	scraper.Debugf(1, "%s: %d scenes discovered, fetching details with %d workers", siteID, len(entries), opts.Workers)
	if !send(ctx, out, scraper.Progress(len(entries))) {
		return
	}
	s.fetchDetails(ctx, studioURL, base, entries, opts, out)
}

func (s *Scraper) fetchDetails(ctx context.Context, studioURL, base string, entries []listEntry, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	work := make(chan listEntry)
	var wg sync.WaitGroup
	// LIFO: close(work) ends the workers' range loops, then wg.Wait blocks
	// until they are gone, so a ctx.Done bail below cannot leak them.
	defer wg.Wait()
	defer close(work)

	for i := 0; i < opts.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for e := range work {
				if !sleep(ctx, opts.Delay) {
					return
				}
				scene := s.buildScene(ctx, studioURL, base, e, out)
				if !send(ctx, out, scraper.Scene(scene)) {
					return
				}
			}
		}()
	}

	for _, e := range entries {
		select {
		case work <- e:
		case <-ctx.Done():
			return
		}
	}
}

// buildScene fetches the detail page for the fields the card does not carry.
// A failed or unparseable detail page costs the description and the brand, not
// the scene — the card already holds id, title, date, performers and thumb, so
// dropping it would lose more than it protects.
func (s *Scraper) buildScene(ctx context.Context, studioURL, base string, e listEntry, out chan<- scraper.SceneResult) models.Scene {
	sceneURL := base + e.path
	scene := models.Scene{
		ID:         e.id,
		SiteID:     siteID,
		StudioURL:  studioURL,
		Title:      e.title,
		URL:        sceneURL,
		Studio:     studioName,
		Performers: e.performers,
		Thumbnail:  resolveURL(base, e.thumb),
		Date:       e.date,
		ScrapedAt:  time.Now().UTC(),
	}

	body, err := s.fetch(ctx, sceneURL)
	if err != nil {
		send(ctx, out, scraper.Error(fmt.Errorf("detail %s: %w", e.id, err)))
		return scene
	}
	d := parseDetail(string(body))
	if d.title == "" && d.description == "" && d.site == "" {
		send(ctx, out, scraper.Error(scraper.ParseError(sceneURL, fmt.Errorf("no scene block on detail page"))))
		return scene
	}
	if d.title != "" {
		scene.Title = d.title
	}
	scene.Description = d.description
	scene.Series = d.site
	if d.preview != "" {
		scene.Preview = resolveURL(base, d.preview)
	}
	if d.poster != "" {
		scene.Thumbnail = resolveURL(base, d.poster)
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

// ---- listing cards ----

type listEntry struct {
	id         string
	path       string
	title      string
	performers []string
	date       time.Time
	thumb      string
}

var (
	// Category pages lay cards out four to a row (`col-sm-3`), model pages two
	// (`col-sm-6`); the trailing classes are what actually mark a card.
	cardSplitRe = regexp.MustCompile(`<div class="col-sm-\d+ pad_bottom_15 text-center">`)
	trailerRe   = regexp.MustCompile(`href="(/?(?:tour3/)?trailers/[^"?#]+\.html)"`)
	cardThumbRe = regexp.MustCompile(`<img[^>]+src="([^"]+)"`)
	// The thumbnail path carries the CMS set directory (`content/kill499/3.jpg`).
	// It is the network's own identifier and survives a title rename, which the
	// slug does not, so it is preferred as the scene id.
	setDirRe    = regexp.MustCompile(`content/([A-Za-z0-9_-]+)/`)
	cardTitleRe = regexp.MustCompile(`(?s)<h3><a[^>]*>(.*?)</a>`)
	cardDateRe  = regexp.MustCompile(`<p>\s*([A-Z][a-z]+ \d{1,2}, \d{4})\s*</p>`)
	cardModelRe = regexp.MustCompile(`(?s)<p class="categories">(.*?)</p>`)
	anchorRe    = regexp.MustCompile(`(?s)<a[^>]*>(.*?)</a>`)
	slugRe      = regexp.MustCompile(`/([^/]+)\.html$`)
)

func parseCards(body string) []listEntry {
	chunks := cardSplitRe.Split(body, -1)
	var out []listEntry
	for i, chunk := range chunks {
		if i == 0 {
			continue // preamble before the first card
		}
		m := trailerRe.FindStringSubmatch(chunk)
		if m == nil {
			continue
		}
		e := listEntry{
			path:  normalizePath(m[1]),
			title: cleanText(firstSubmatch(cardTitleRe, chunk)),
			thumb: firstSubmatch(cardThumbRe, chunk),
		}
		e.id = sceneID(e.thumb, e.path)
		if e.id == "" {
			continue
		}
		if d, ok := parseDate(firstSubmatch(cardDateRe, chunk)); ok {
			e.date = d
		}
		if span := firstSubmatch(cardModelRe, chunk); span != "" {
			e.performers = anchorTexts(span)
		}
		out = append(out, e)
	}
	return out
}

func sceneID(thumb, path string) string {
	if m := setDirRe.FindStringSubmatch(thumb); m != nil {
		return m[1]
	}
	return firstSubmatch(slugRe, path)
}

// normalizePath makes a card's href absolute against the site root. Cards on
// category pages write `/tour3/trailers/…`; model pages write it relative.
func normalizePath(href string) string {
	if strings.HasPrefix(href, "/") {
		return href
	}
	return "/tour3/" + strings.TrimPrefix(href, "tour3/")
}

// ---- detail page ----

type detail struct {
	title       string
	description string
	site        string
	poster      string
	preview     string
}

var (
	detailTitleRe = regexp.MustCompile(`(?s)<h2>(.*?)</h2>`)
	detailSiteRe  = regexp.MustCompile(`(?s)Site:\s*<b>(.*?)</b>`)
	detailDescRe  = regexp.MustCompile(`(?s)<p class="description">(.*?)</p>`)
	posterRe      = regexp.MustCompile(`poster="([^"]+)"`)
	sourceRe      = regexp.MustCompile(`<source[^>]+src="([^"]+)"`)
)

func parseDetail(body string) detail {
	return detail{
		title:       cleanText(firstSubmatch(detailTitleRe, body)),
		description: cleanText(firstSubmatch(detailDescRe, body)),
		site:        cleanText(firstSubmatch(detailSiteRe, body)),
		poster:      firstSubmatch(posterRe, body),
		preview:     firstSubmatch(sourceRe, body),
	}
}

// ---- helpers ----

var tagStripRe = regexp.MustCompile(`<[^>]*>`)

func cleanText(s string) string {
	s = tagStripRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, " ", " ")
	return strings.Join(strings.Fields(s), " ")
}

func anchorTexts(span string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, m := range anchorRe.FindAllStringSubmatch(span, -1) {
		t := cleanText(m[1])
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

func firstSubmatch(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func parseDate(s string) (time.Time, bool) {
	t, err := time.Parse("January 2, 2006", s)
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

// resolveURL absolutizes the tour's own asset paths. Thumbnails are written
// against trans500.com itself, so a test server's origin has to win over the
// absolute form the page embeds.
func resolveURL(base, ref string) string {
	switch {
	case ref == "":
		return ""
	case strings.HasPrefix(ref, "/"):
		return base + ref
	case strings.HasPrefix(ref, "http://www.trans500.com"):
		return base + strings.TrimPrefix(ref, "http://www.trans500.com")
	case strings.HasPrefix(ref, "https://www.trans500.com"):
		return base + strings.TrimPrefix(ref, "https://www.trans500.com")
	case strings.HasPrefix(ref, "http://"), strings.HasPrefix(ref, "https://"):
		return ref
	default:
		return base + "/tour3/" + ref
	}
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
