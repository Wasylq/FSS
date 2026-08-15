// Package heyzo scrapes heyzo.com, a static Japanese uncensored-video site.
//
// Despite the D2Pass billing link on every page it is not a D2Pass "bifrost"
// site: the JSON endpoints d2passutil drives (`/dyn/phpauto/movie_lists/…`)
// answer 404 here. What it has instead is plain HTML — `/listpages/all_{N}.html`
// pages of 30 cards, and a `/moviepages/{id}/index.html` per movie.
//
// Discovery walks the listing; the cards already carry id, title, thumbnail,
// performer and release date. The detail page is fetched for what they do not
// carry: a schema.org `Movie` JSON-LD block with the description and an ISO
// duration, plus a `movieInfo` table holding the series and the tag keywords.
//
// Filtered views work as listings too — `/listpages/actor_{id}_{N}.html` for
// one performer and `/listpages/category_{id}_{N}.html` for a category — so a
// URL naming either scrapes that slice with the same walk.
package heyzo

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
	siteID     = "heyzo"
	domain     = "heyzo.com"
	studioName = "HEYZO"
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
	matchRe = regexp.MustCompile(`^https?://(?:www\.|en\.)?heyzo\.com(?:/|$)`)
	// A filtered listing names its own stem: `actor_293_1.html` walks one
	// performer, `category_22_1.html` one category. The trailing number is the
	// page, so it is dropped and rebuilt.
	filterRe = regexp.MustCompile(`/listpages/((?:actor|category|series)_\d+)_\d+\.html`)
)

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		domain,
		domain + "/listpages/all_{N}.html",
		domain + "/listpages/actor_{id}_{N}.html",
		domain + "/listpages/category_{id}_{N}.html",
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

// base resolves the origin to fetch from. The operator's own host wins so the
// en. mirror stays addressable; baseOverride is the test server.
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
	stem := "all"
	if m := filterRe.FindStringSubmatch(studioURL); m != nil {
		stem = m[1]
		scraper.Debugf(1, "%s: scraping filtered listing %s", siteID, stem)
	} else {
		scraper.Debugf(1, "%s: scraping the full catalogue", siteID)
	}

	// Discovery and detail fetching run together: the catalogue is ~115
	// listing pages, and holding every scene back until the last one is read
	// leaves a scrape looking hung for half a minute before the first result.
	// The consequence is that the true count is only known at the end, so the
	// progress total is sent then — the cmd layer shows a running count until
	// it arrives.
	work := make(chan listEntry)
	var wg sync.WaitGroup
	for i := 0; i < opts.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.detailWorker(ctx, studioURL, base, work, opts, out)
		}()
	}

	found := s.discover(ctx, base, stem, opts, out, work)
	close(work)
	wg.Wait()

	if found > 0 {
		send(ctx, out, scraper.Progress(found))
	}
}

// maxListingPages bounds the walk. The catalogue is ~115 pages; the cap is a
// runaway guard, since a page past the end answers 200 with no cards rather
// than 404ing and an unbounded loop would depend entirely on that parse.
const maxListingPages = 2000

// discover walks the listing, handing each new movie to the detail pool as it
// is seen. It returns how many it sent.
func (s *Scraper) discover(ctx context.Context, base, stem string, opts scraper.ListOpts, out chan<- scraper.SceneResult, work chan<- listEntry) int {
	seen := make(map[string]bool)
	sent := 0

	for page := 1; page <= maxListingPages; page++ {
		if ctx.Err() != nil {
			return sent
		}
		if page > 1 && !sleep(ctx, opts.Delay) {
			return sent
		}

		pageURL := fmt.Sprintf("%s/listpages/%s_%d.html", base, stem, page)
		scraper.Debugf(1, "%s: fetching listing page %d", siteID, page)
		body, err := s.fetch(ctx, pageURL)
		if err != nil {
			send(ctx, out, scraper.Error(fmt.Errorf("listing page %d: %w", page, err)))
			return sent
		}

		cards := parseCards(string(body))
		if len(cards) == 0 {
			if page == 1 {
				// Past the last page the site answers 200 with an empty grid,
				// so an empty page is the ordinary end signal — except on the
				// first one, where it means the template changed or the filter
				// does not exist. That has to be loud, or an authoritative
				// --full save reads it as a catalogue that vanished.
				send(ctx, out, scraper.Error(scraper.ParseError(pageURL, fmt.Errorf("no movie cards on the first listing page"))))
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
				// The listing is newest-first, so the first stored id means
				// everything after it is stored too.
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
		if !send(ctx, out, scraper.Scene(s.buildScene(ctx, studioURL, base, e, out))) {
			return
		}
	}
}

// buildScene fetches the detail page for the fields the card does not carry. A
// failed detail costs the description, duration and tags, not the scene: the
// card already holds id, title, date, performer and thumbnail.
func (s *Scraper) buildScene(ctx context.Context, studioURL, base string, e listEntry, out chan<- scraper.SceneResult) models.Scene {
	sceneURL := base + e.path
	scene := models.Scene{
		ID:        e.id,
		SiteID:    siteID,
		StudioURL: studioURL,
		Title:     e.title,
		URL:       sceneURL,
		Studio:    studioName,
		Thumbnail: absolutize(e.thumb, base),
		Date:      e.date,
		ScrapedAt: time.Now().UTC(),
	}
	if e.performer != "" {
		scene.Performers = []string{e.performer}
	}

	body, err := s.fetch(ctx, sceneURL)
	if err != nil {
		send(ctx, out, scraper.Error(fmt.Errorf("detail %s: %w", e.id, err)))
		return scene
	}
	d := parseDetail(string(body))
	if d.empty() {
		send(ctx, out, scraper.Error(scraper.ParseError(sceneURL, fmt.Errorf("no movie block on detail page"))))
		return scene
	}
	if d.title != "" {
		scene.Title = d.title
	}
	if d.description != "" {
		scene.Description = d.description
	}
	if d.duration > 0 {
		scene.Duration = d.duration
	}
	if !d.date.IsZero() {
		scene.Date = d.date
	}
	if len(d.performers) > 0 {
		scene.Performers = d.performers
	}
	scene.Tags = d.tags
	scene.Categories = d.categories
	scene.Series = d.series
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
	id        string
	path      string
	title     string
	thumb     string
	performer string
	date      time.Time
}

var (
	cardSplitRe = regexp.MustCompile(`<div\s+class="movie movie\d+`)
	movieIDRe   = regexp.MustCompile(`data-movie-id="(\d+)"`)
	cardPathRe  = regexp.MustCompile(`href="(/moviepages/\d+/index\.html)"`)
	cardTitleRe = regexp.MustCompile(`title="([^"]*)"\s*/?>`)
	cardThumbRe = regexp.MustCompile(`data-original="([^"]+)"`)
	cardActorRe = regexp.MustCompile(`(?s)class="actor"[^>]*>(.*?)</a>`)
	cardAltRe   = regexp.MustCompile(`alt="([^"]*)"`)
	cardDateRe  = regexp.MustCompile(`(?s)class="release">.*?(\d{4}-\d{2}-\d{2})`)
)

func parseCards(body string) []listEntry {
	chunks := cardSplitRe.Split(body, -1)
	var out []listEntry
	for i, chunk := range chunks {
		if i == 0 {
			continue // preamble before the first card
		}
		id := firstSubmatch(movieIDRe, chunk)
		path := firstSubmatch(cardPathRe, chunk)
		if id == "" || path == "" {
			continue
		}
		e := listEntry{
			id:        id,
			path:      path,
			title:     cleanText(firstSubmatch(cardTitleRe, chunk)),
			thumb:     firstSubmatch(cardThumbRe, chunk),
			performer: cleanText(firstSubmatch(cardActorRe, chunk)),
		}
		// The thumbnail's alt text is the performer; it is the fallback for a
		// card whose actor link is missing.
		if e.performer == "" {
			e.performer = cleanText(firstSubmatch(cardAltRe, chunk))
		}
		if d, ok := parseDate(firstSubmatch(cardDateRe, chunk)); ok {
			e.date = d
		}
		out = append(out, e)
	}
	return out
}

// ---- detail page ----

type detail struct {
	title       string
	description string
	duration    int
	date        time.Time
	performers  []string
	tags        []string
	categories  []string
	series      string
	// foundTable records that the movieInfo table was located, which is the
	// signal that the page parsed at all. The JSON-LD block is absent on the
	// newer pages, so its absence alone says nothing.
	foundTable bool
}

func (d detail) empty() bool {
	return !d.foundTable && d.title == "" && d.description == "" && d.duration == 0
}

// cellRe matches one table cell. A movieInfo row is a label cell followed by a
// value cell, so only the last match carries data.
var cellRe = regexp.MustCompile(`(?s)<td[^>]*>(.*?)</td>`)

func valueCell(row string) string {
	cells := cellRe.FindAllStringSubmatch(row, -1)
	if len(cells) == 0 {
		return ""
	}
	return cells[len(cells)-1][1]
}

var (
	ldJSONRe     = regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)
	scriptRe     = regexp.MustCompile(`(?s)<script.*?</script>`)
	infoTableRe  = regexp.MustCompile(`(?s)<table class="movieInfo">(.*?)</table>`)
	rowRe        = regexp.MustCompile(`(?s)<tr class="([^"]*)">(.*?)</tr>`)
	anchorTextRe = regexp.MustCompile(`(?s)<a[^>]*>(.*?)</a>`)
)

func parseDetail(body string) detail {
	var d detail

	if m := ldJSONRe.FindStringSubmatch(body); m != nil {
		ld := parseMovieLD(m[1])
		d.title = ld.title
		d.description = ld.description
		d.duration = ld.duration
		d.date = ld.date
		if ld.actor != "" {
			d.performers = []string{ld.actor}
		}
	}

	// Scripts carry jQuery selectors naming the very row classes below, so
	// they are stripped before the table is located.
	table := firstSubmatch(infoTableRe, scriptRe.ReplaceAllString(body, ""))
	if table == "" {
		return d
	}
	d.foundTable = true
	for _, row := range rowRe.FindAllStringSubmatch(table, -1) {
		class, cells := row[1], row[2]
		switch {
		case strings.Contains(class, "table-actor-type"):
			d.categories = anchorTexts(cells)
		case strings.Contains(class, "table-tag-keyword"):
			d.tags = append(d.tags, anchorTexts(cells)...)
		case strings.Contains(class, "table-series"):
			// The row is label cell then value cell, so only the last one is
			// the series. An absent series is rendered as a run of dashes.
			if v := cleanText(valueCell(cells)); v != "" && strings.Trim(v, "-") != "" {
				d.series = v
			}
		case strings.Contains(class, "table-actor"):
			if names := anchorTexts(cells); len(names) > 0 {
				d.performers = names
			}
		case strings.Contains(class, "table-memo"):
			// The `memo` paragraph is the site's own synopsis and is present
			// on every page, including the newer ones that ship no JSON-LD at
			// all — 243 of the first 963 scenes had no description without it.
			if v := cleanText(valueCell(cells)); v != "" {
				d.description = v
			}
		}
	}
	d.tags = dedupe(d.tags)
	return d
}

type movieLD struct {
	title       string
	description string
	duration    int
	date        time.Time
	actor       string
}

var (
	ldNameRe = regexp.MustCompile(`"name"\s*:\s*"((?:[^"\\]|\\.)*)"`)
	ldDescRe = regexp.MustCompile(`"description"\s*:\s*"((?:[^"\\]|\\.)*)"`)
	ldDurRe  = regexp.MustCompile(`"duration"\s*:\s*"([^"]+)"`)
	ldDateRe = regexp.MustCompile(`"dateCreated"\s*:\s*"(\d{4}-\d{2}-\d{2})"`)
	// The actor object nests a name, so the first "name" after "actor" is it.
	ldActorRe = regexp.MustCompile(`(?s)"actor"\s*:\s*\{.*?"name"\s*:\s*"((?:[^"\\]|\\.)*)"`)
)

// parseMovieLD reads the fields we want out of the schema.org Movie block by
// pattern rather than by decoding it. The block nests the same keys under both
// the Movie and its `video` MediaObject, and the outer copy is the one that
// comes first — decoding into a struct would need the whole shape declared to
// get at four fields.
func parseMovieLD(s string) movieLD {
	var ld movieLD
	ld.title = cleanText(unescapeJSON(firstSubmatch(ldNameRe, s)))
	ld.description = cleanText(unescapeJSON(firstSubmatch(ldDescRe, s)))
	ld.actor = cleanText(unescapeJSON(firstSubmatch(ldActorRe, s)))
	if v := firstSubmatch(ldDurRe, s); v != "" {
		ld.duration = parseutil.ParseDurationISO(v)
	}
	if d, ok := parseDate(firstSubmatch(ldDateRe, s)); ok {
		ld.date = d
	}
	return ld
}

// ---- helpers ----

var tagStripRe = regexp.MustCompile(`<[^>]*>`)

func stripTags(s string) string { return tagStripRe.ReplaceAllString(s, " ") }

func cleanText(s string) string {
	s = stripTags(s)
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, " ", " ")
	return strings.Join(strings.Fields(s), " ")
}

// unescapeJSON turns the escapes a JSON string literal may carry into their
// characters. Only the ones that appear in this payload are handled; anything
// else is left alone rather than corrupted.
func unescapeJSON(s string) string {
	return strings.NewReplacer(`\"`, `"`, `\\`, `\`, `\n`, " ", `\r`, " ", `\t`, " ", `\/`, "/").Replace(s)
}

func anchorTexts(s string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, m := range anchorTextRe.FindAllStringSubmatch(s, -1) {
		t := cleanText(m[1])
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

func dedupe(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(in))
	out := in[:0:0]
	for _, v := range in {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
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
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

// absolutize resolves the site's protocol-relative and rooted asset paths.
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
