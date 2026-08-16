// Package bbwhighway scrapes bbwhighway.com.
//
// The tour is a NATS theme close to the Extreme Movie Pass one — `modelfeature`
// cards with `set-target-{sceneID}-{thumbID}` thumbnails — but with two
// differences that make reusing that util wrong: the card's own anchor text is
// truncated (`HER FAVORITE TOY PLUS...`) with the full title only in the
// `title` attribute, and it names no performers at all. Unlike the
// affiliate-only sister sites this one has real `/tour/trailers/{slug}.html`
// pages, which carry the cast, so discovery is followed by a detail fetch.
//
// The listing paginates at `/tour/categories/movies_{N}_d.html`, 12 a page.
package bbwhighway

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
	"github.com/Anastylosis/FSS/parseutil"
	"github.com/Anastylosis/FSS/scraper"
)

const (
	siteID     = "bbwhighway"
	domain     = "bbwhighway.com"
	studioName = "BBW Highway"
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
	matchRe = regexp.MustCompile(`^https?://(?:www\.)?bbwhighway\.com(?:/|$)`)
	sceneRe = regexp.MustCompile(`/tour/trailers/([^/?#]+)\.html`)
)

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		domain,
		domain + "/tour/",
		domain + "/tour/trailers/{slug}.html",
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
	return "https://" + domain
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
	// the first scene lands after two requests instead of the whole walk.
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

// maxListingPages bounds the walk; the catalogue is ~42 pages.
const maxListingPages = 500

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
				// The listing is date-sorted (`_d`), so a stored id means the
				// rest of the walk is stored too.
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

// buildScene fetches the scene's own page for the cast, which the card does not
// carry. The card's id, title, date, runtime and thumbnail are the fallback for
// a detail that fails.
func (s *Scraper) buildScene(ctx context.Context, studioURL, base string, e listEntry, out chan<- scraper.SceneResult) models.Scene {
	sceneURL := base + "/tour/trailers/" + e.slug + ".html"
	scene := models.Scene{
		ID:        e.id,
		SiteID:    siteID,
		StudioURL: studioURL,
		Title:     e.title,
		URL:       sceneURL,
		Studio:    studioName,
		Thumbnail: absolutize(e.thumb, base),
		Date:      e.date,
		Duration:  e.duration,
		ScrapedAt: time.Now().UTC(),
	}

	body, err := s.fetch(ctx, sceneURL)
	if err != nil {
		send(ctx, out, scraper.Error(fmt.Errorf("scene %s: %w", e.slug, err)))
		return orEmpty(scene)
	}
	d := parseDetail(string(body))
	if d.title == "" && len(d.performers) == 0 {
		send(ctx, out, scraper.Error(scraper.ParseError(sceneURL, fmt.Errorf("no scene block on the page"))))
		return orEmpty(scene)
	}
	if d.title != "" {
		scene.Title = d.title
	}
	scene.Performers = d.performers
	scene.Categories = d.categories
	if !d.date.IsZero() {
		scene.Date = d.date
	}
	if d.duration > 0 {
		scene.Duration = d.duration
	}
	if scene.ID == "" {
		scene.ID = d.setID
	}
	if scene.ID == "" {
		scene.ID = e.slug
	}
	// The tour's "description" is the title repeated, so it is not stored —
	// a description identical to the title is noise downstream.
	return scene
}

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
	id       string
	slug     string
	title    string
	thumb    string
	date     time.Time
	duration int
}

var (
	cardSplitRe = regexp.MustCompile(`<div class="modelfeature[^"]*">`)
	cardIDRe    = regexp.MustCompile(`id="set-target-(\d+)-\d+"`)
	cardHrefRe  = regexp.MustCompile(`href="[^"]*/tour/trailers/([^"/?#]+)\.html"`)
	// The anchor text is truncated; the title attribute is the full one.
	cardTitleRe = regexp.MustCompile(`<a[^>]+href="[^"]*/tour/trailers/[^"]*"[^>]*title="([^"]*)"`)
	cardThumbRe = regexp.MustCompile(`src0_1x="([^"]+)"`)
	// "25:20 | 2026-08-17"
	cardMetaRe = regexp.MustCompile(`<p>\s*(\d{1,2}:\d{2}(?::\d{2})?)\s*\|\s*(\d{4}-\d{2}-\d{2})\s*</p>`)
)

func parseCards(body string) []listEntry {
	chunks := cardSplitRe.Split(body, -1)
	var out []listEntry
	for i, chunk := range chunks {
		if i == 0 {
			continue // preamble before the first card
		}
		e := listEntry{
			id:    firstSubmatch(cardIDRe, chunk),
			slug:  firstSubmatch(cardHrefRe, chunk),
			title: cleanText(firstSubmatch(cardTitleRe, chunk)),
			thumb: firstSubmatch(cardThumbRe, chunk),
		}
		if e.id == "" || e.slug == "" {
			continue
		}
		if m := cardMetaRe.FindStringSubmatch(chunk); m != nil {
			e.duration = parseutil.ParseDurationColon(m[1])
			if t, err := time.Parse("2006-01-02", m[2]); err == nil {
				e.date = t.UTC()
			}
		}
		out = append(out, e)
	}
	return out
}

// ---- detail page ----

type detail struct {
	title      string
	performers []string
	categories []string
	date       time.Time
	duration   int
	setID      string
}

var (
	scriptRe   = regexp.MustCompile(`(?s)<script.*?</script>`)
	h1Re       = regexp.MustCompile(`(?s)<h1>(.*?)</h1>`)
	modelsRe   = regexp.MustCompile(`(?s)<span class="tour_update_models">(.*?)</span>`)
	anchorRe   = regexp.MustCompile(`(?s)<a[^>]*>(.*?)</a>`)
	detailIDRe = regexp.MustCompile(`id="set-target-(\d+)-\d+"`)
	// "08/17/2026 | 25:20 | Categories: <a>x</a>, <a>y</a>"
	dateLineRe = regexp.MustCompile(`(?s)<p class="date">(.*?)</p>`)
	usDateRe   = regexp.MustCompile(`(\d{2}/\d{2}/\d{4})`)
	runtimeRe  = regexp.MustCompile(`(\d{1,2}:\d{2}(?::\d{2})?)`)
)

func parseDetail(body string) detail {
	body = scriptRe.ReplaceAllString(body, "")
	d := detail{
		title: cleanText(firstSubmatch(h1Re, body)),
		setID: firstSubmatch(detailIDRe, body),
	}
	for _, m := range anchorRe.FindAllStringSubmatch(firstSubmatch(modelsRe, body), -1) {
		if n := cleanText(m[1]); n != "" {
			d.performers = appendUnique(d.performers, n)
		}
	}

	line := firstSubmatch(dateLineRe, body)
	if v := firstSubmatch(usDateRe, line); v != "" {
		if t, err := time.Parse("01/02/2006", v); err == nil {
			d.date = t.UTC()
		}
	}
	if v := firstSubmatch(runtimeRe, line); v != "" {
		d.duration = parseutil.ParseDurationColon(v)
	}
	// Categories follow the "Categories:" label on the same line; the label is
	// present even when the list behind it is empty.
	if _, after, ok := strings.Cut(line, "Categories:"); ok {
		for _, m := range anchorRe.FindAllStringSubmatch(after, -1) {
			if c := cleanText(m[1]); c != "" {
				d.categories = appendUnique(d.categories, c)
			}
		}
	}
	return d
}

// ---- helpers ----

var tagStripRe = regexp.MustCompile(`<[^>]*>`)

func cleanText(s string) string {
	s = tagStripRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
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

func absolutize(ref, base string) string {
	switch {
	case ref == "":
		return ""
	case strings.HasPrefix(ref, "//"):
		scheme := "https:"
		if i := strings.Index(base, "//"); i > 0 {
			scheme = base[:i]
		}
		return scheme + ref
	case strings.HasPrefix(ref, "/"):
		return base + ref
	default:
		return ref
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
