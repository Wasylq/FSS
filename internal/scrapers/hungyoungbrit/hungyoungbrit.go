// Package hungyoungbrit scrapes hungyoungbrit.com.
//
// The tour is a Bootstrap rework of an Elevated X listing: cards are
// `video-thumb` divs carrying a `data-setid` and an `h3.scene-title`, paginated
// at `/tour/categories/movies_{N}_d.html`, 12 a page over 27 pages. Past the
// last page the listing serves a card-free page, which is a clean stop.
//
// Each scene's `/tour/updates/{slug}.html` page carries the full title, the
// cast, the runtime, the release date and a rating. **Its synopsis does not
// survive** — the tour ships the full text only inside an HTML comment and
// leaves a truncated copy in the meta tags, so the stored description is the
// truncated one rather than markup that is not really on the page.
//
// Thumbnails are signed CDN URLs (`?expires=…&token=…`) that expire within
// hours, which `internal/mediafetch` already knows how to spot; nothing here
// tries to keep them fresh.
package hungyoungbrit

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Anastylosis/FSS/internal/httpx"
	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/scraper"
)

const (
	siteID     = "hungyoungbrit"
	domain     = "hungyoungbrit.com"
	studioName = "Hung Young Brit"
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
	matchRe = regexp.MustCompile(`^https?://(?:www\.)?hungyoungbrit\.com(?:/|$)`)
	sceneRe = regexp.MustCompile(`/tour/updates/([^/?#]+)\.html`)
)

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		domain,
		domain + "/tour/categories/movies.html",
		domain + "/tour/updates/{slug}.html",
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
// non-www spelling stays addressable; baseOverride is the test server. Every
// request is built as base + path, never from the absolute URLs the cards
// embed, so pointing base at httptest redirects the whole crawl.
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

	// A single scene URL is a legitimate thing to point at.
	if m := sceneRe.FindStringSubmatch(studioURL); m != nil {
		scraper.Debugf(1, "%s: scraping one scene %s", siteID, m[1])
		if !send(ctx, out, scraper.Progress(1)) {
			return
		}
		if sc := s.buildScene(ctx, studioURL, base, listEntry{slug: m[1]}, out); sc.ID != "" {
			send(ctx, out, scraper.Scene(sc))
		}
		return
	}

	scraper.Debugf(1, "%s: walking the movies listing", siteID)

	// Discovery streams into the detail pool rather than completing first, so
	// the first scene lands after two requests instead of 27.
	work := make(chan listEntry)
	var wg sync.WaitGroup
	for i := 0; i < opts.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.detailWorker(ctx, studioURL, base, work, opts, out)
		}()
	}

	found := s.discover(ctx, base, opts, out, work)
	close(work)
	wg.Wait()

	if found > 0 {
		send(ctx, out, scraper.Progress(found))
	}
}

// maxListingPages bounds the walk. The catalogue is 27 pages; the cap is a
// runaway guard.
const maxListingPages = 400

// discover walks the listing, handing each new scene to the detail pool as it
// is seen. It returns how many it sent.
func (s *Scraper) discover(ctx context.Context, base string, opts scraper.ListOpts, out chan<- scraper.SceneResult, work chan<- listEntry) int {
	seen := make(map[string]bool)
	sent := 0

	for page := 1; page <= maxListingPages; page++ {
		if ctx.Err() != nil {
			return sent
		}
		if page > 1 && !sleep(ctx, opts.Delay) {
			return sent
		}

		pageURL := fmt.Sprintf("%s/tour/categories/movies_%d_d.html", base, page)
		scraper.Debugf(1, "%s: fetching listing page %d", siteID, page)
		body, err := s.fetch(ctx, pageURL)
		if err != nil {
			send(ctx, out, scraper.Error(fmt.Errorf("listing page %d: %w", page, err)))
			return sent
		}

		cards := parseCards(string(body))
		if len(cards) == 0 {
			if page == 1 {
				// A listing that fetched cleanly and parsed to nothing is a
				// template change, not an empty catalogue. Saying so keeps an
				// authoritative --full save from reading the silence as a
				// catalogue that vanished.
				send(ctx, out, scraper.Error(scraper.ParseError(pageURL, fmt.Errorf("no scene cards on the first listing page"))))
			}
			return sent
		}

		added := 0
		for _, c := range cards {
			if seen[c.id] {
				continue
			}
			seen[c.id] = true
			added++
			if opts.KnownIDs[c.id] {
				// The listing runs newest-first — set ids descend across
				// pages — so a stored id means the rest is stored too.
				scraper.Debugf(1, "%s: hit known ID %s, stopping early", siteID, c.id)
				send(ctx, out, scraper.StoppedEarly())
				return sent
			}
			select {
			case work <- c:
				sent++
			case <-ctx.Done():
				return sent
			}
		}
		if added == 0 {
			return sent
		}
	}
	return sent
}

func (s *Scraper) detailWorker(ctx context.Context, studioURL, base string, work <-chan listEntry, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	for e := range work {
		if !sleep(ctx, opts.Delay) {
			return
		}
		sc := s.buildScene(ctx, studioURL, base, e, out)
		if sc.ID == "" {
			continue
		}
		if !send(ctx, out, scraper.Scene(sc)) {
			return
		}
	}
}

// buildScene fetches the scene's own page. The card's id, title and thumbnail
// are the fallback for a detail that fails, so a failure costs the cast, date
// and runtime rather than the scene.
func (s *Scraper) buildScene(ctx context.Context, studioURL, base string, e listEntry, out chan<- scraper.SceneResult) models.Scene {
	sceneURL := base + "/tour/updates/" + e.slug + ".html"
	scene := models.Scene{
		ID:        e.id,
		SiteID:    siteID,
		StudioURL: studioURL,
		Title:     e.title,
		URL:       sceneURL,
		Studio:    studioName,
		Thumbnail: e.thumb,
		ScrapedAt: time.Now().UTC(),
	}

	body, err := s.fetch(ctx, sceneURL)
	if err != nil {
		send(ctx, out, scraper.Error(fmt.Errorf("scene %s: %w", e.slug, err)))
		return orEmpty(scene)
	}
	d := parseDetail(string(body))
	if d.title == "" && len(d.performers) == 0 {
		send(ctx, out, scraper.Error(scraper.ParseError(sceneURL, fmt.Errorf("no scene panel on the page"))))
		return orEmpty(scene)
	}
	if d.title != "" {
		scene.Title = d.title
	}
	scene.Description = d.description
	scene.Performers = d.performers
	scene.Duration = d.duration
	scene.Date = d.date
	if d.thumb != "" && scene.Thumbnail == "" {
		scene.Thumbnail = d.thumb
	}
	// The set id is only on the listing card; a single-scene run has to take it
	// from the page's own thumbnail path.
	if scene.ID == "" {
		scene.ID = d.setID
	}
	if scene.ID == "" {
		scene.ID = e.slug
	}
	return scene
}

// orEmpty drops a scene with neither an id nor a title, which is what a
// single-scene run produces when its only fetch failed.
func orEmpty(s models.Scene) models.Scene {
	if s.Title == "" && s.ID == "" {
		return models.Scene{}
	}
	return s
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
	id    string
	slug  string
	title string
	thumb string
}

var (
	cardSplitRe = regexp.MustCompile(`<div class="[^"]*video-thumb"[^>]*data-setid="(\d+)"`)
	cardHrefRe  = regexp.MustCompile(`href="[^"]*/tour/updates/([^"/?#]+)\.html"`)
	cardTitleRe = regexp.MustCompile(`(?s)<h3 class="scene-title">(.*?)</h3>`)
	cardThumbRe = regexp.MustCompile(`src0_1x="([^"]+)"`)
)

func parseCards(body string) []listEntry {
	locs := cardSplitRe.FindAllStringSubmatchIndex(body, -1)
	out := make([]listEntry, 0, len(locs))
	for i, loc := range locs {
		end := len(body)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		chunk := body[loc[0]:end]
		e := listEntry{
			id:    body[loc[2]:loc[3]],
			slug:  firstSubmatch(cardHrefRe, chunk),
			title: cleanText(firstSubmatch(cardTitleRe, chunk)),
			thumb: firstSubmatch(cardThumbRe, chunk),
		}
		if e.slug == "" {
			continue
		}
		out = append(out, e)
	}
	return out
}

// ---- detail page ----

type detail struct {
	title       string
	description string
	performers  []string
	duration    int
	date        time.Time
	thumb       string
	setID       string
}

var (
	// The tour ships a second, commented-out copy of the whole scene panel.
	// Stripping comments first is what keeps the parser off markup that is not
	// really on the page — the commented copy even carries a fuller synopsis.
	commentRe  = regexp.MustCompile(`(?s)<!--.*?-->`)
	scriptRe   = regexp.MustCompile(`(?s)<script.*?</script>`)
	titleRe    = regexp.MustCompile(`(?s)<h3 class=['"]titleHYB['"]>(.*?)</h3>`)
	modelsRe   = regexp.MustCompile(`(?s)<span class="update_models">(.*?)</span>`)
	anchorRe   = regexp.MustCompile(`(?s)<a[^>]*>(.*?)</a>`)
	lengthRe   = regexp.MustCompile(`Scene Length:\s*(\d+)`)
	releaseRe  = regexp.MustCompile(`Release Date:\s*(\d{4}-\d{2}-\d{2})`)
	ogDescRe   = regexp.MustCompile(`<meta property="og:description" content="([^"]*)"`)
	contentRe  = regexp.MustCompile(`src="([^"]*/tour/content/[^"]+)"`)
	setPathRe  = regexp.MustCompile(`/tour/content/(?:contentthumbs/)?(?:\d+/\d+/)?(\d+)-\d+x\.jpg`)
	sceneTitle = regexp.MustCompile(`<meta property="og:title" content="([^"]*)"`)
)

func parseDetail(body string) detail {
	// og: tags are read before comments are stripped only because they are in
	// <head> and never commented; everything else comes from the live body.
	ogTitle := cleanText(firstSubmatch(sceneTitle, body))
	ogDesc := cleanText(firstSubmatch(ogDescRe, body))

	live := commentRe.ReplaceAllString(scriptRe.ReplaceAllString(body, ""), "")

	d := detail{
		title:       cleanText(firstSubmatch(titleRe, live)),
		description: ogDesc,
		thumb:       firstSubmatch(contentRe, live),
	}
	if d.title == "" {
		// og:title appends " - HungYoungBrit.com".
		d.title = strings.TrimSuffix(ogTitle, " - HungYoungBrit.com")
	}
	for _, m := range anchorRe.FindAllStringSubmatch(firstSubmatch(modelsRe, live), -1) {
		if n := cleanText(m[1]); n != "" {
			d.performers = appendUnique(d.performers, n)
		}
	}
	if v := firstSubmatch(lengthRe, live); v != "" {
		if mins, err := strconv.Atoi(v); err == nil {
			d.duration = mins * 60
		}
	}
	if v := firstSubmatch(releaseRe, live); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			d.date = t.UTC()
		}
	}
	if m := setPathRe.FindStringSubmatch(live); m != nil {
		d.setID = m[1]
	}
	return d
}

// ---- helpers ----

var tagStripRe = regexp.MustCompile(`<[^>]*>`)

func cleanText(s string) string {
	s = tagStripRe.ReplaceAllString(s, " ")
	// The tour double-encodes the emoji it puts in titles (`&amp;#x1F608;`),
	// so one unescape leaves a numeric entity behind.
	s = html.UnescapeString(html.UnescapeString(s))
	s = strings.ReplaceAll(s, " ", " ")
	return strings.Join(strings.Fields(s), " ")
}

func appendUnique(list []string, v string) []string {
	for _, existing := range list {
		if existing == v {
			return list
		}
	}
	return append(list, v)
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
