// Package tripforfuck scrapes tripforfuck.com.
//
// The catalogue is public at `/member/movie/list/index.html?page={N}` despite
// the `/member/` path — no login is needed for the listing or the detail pages,
// only for the videos themselves.
//
// **The site publishes no absolute date anywhere**: both the card and the
// detail page say "490 days ago". That is subtracted from the scrape time,
// which is stable across runs — the site recomputes the same way — but is
// accurate only to the day.
package tripforfuck

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
	siteID     = "tripforfuck"
	domain     = "tripforfuck.com"
	studioName = "Trip For Fuck"
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
	matchRe = regexp.MustCompile(`^https?://(?:www\.)?tripforfuck\.com(?:/|$)`)
	sceneRe = regexp.MustCompile(`/member/movie/([0-9]+-[0-9]+)/`)
)

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		domain,
		domain + "/member/movie/list/index.html",
		domain + "/member/movie/{id}/index.html",
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
// non-www spelling stays addressable; baseOverride is the test server.
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

	// A single movie URL is a legitimate thing to point at.
	if m := sceneRe.FindStringSubmatch(studioURL); m != nil {
		scraper.Debugf(1, "%s: scraping one movie %s", siteID, m[1])
		if !send(ctx, out, scraper.Progress(1)) {
			return
		}
		if sc := s.buildScene(ctx, studioURL, base, listEntry{id: m[1]}, out); sc.ID != "" {
			send(ctx, out, scraper.Scene(sc))
		}
		return
	}

	scraper.Debugf(1, "%s: walking the movie list", siteID)

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

// maxListingPages bounds the walk; the list runs to five pages today.
const maxListingPages = 200

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

		pageURL := base + "/member/movie/list/index.html"
		if page > 1 {
			pageURL += "?page=" + strconv.Itoa(page)
		}
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
				// The list is newest-first, so a stored id means the rest of
				// the walk is stored too.
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

// buildScene fetches the movie's own page for the cast, description and tags.
// The card's id, title, thumbnail and age are the fallback for a detail that
// fails.
func (s *Scraper) buildScene(ctx context.Context, studioURL, base string, e listEntry, out chan<- scraper.SceneResult) models.Scene {
	sceneURL := base + "/member/movie/" + e.id + "/index.html"
	now := time.Now().UTC()

	scene := models.Scene{
		ID:        e.id,
		SiteID:    siteID,
		StudioURL: studioURL,
		Title:     e.title,
		URL:       sceneURL,
		Studio:    studioName,
		Thumbnail: absolutize(e.thumb, base),
		ScrapedAt: now,
	}
	if e.daysAgo > 0 {
		scene.Date = dateFromDaysAgo(now, e.daysAgo)
	}

	body, err := s.fetch(ctx, sceneURL)
	if err != nil {
		send(ctx, out, scraper.Error(fmt.Errorf("movie %s: %w", e.id, err)))
		return orEmpty(scene)
	}
	d := parseDetail(string(body))
	if d.title == "" && len(d.performers) == 0 && d.description == "" {
		send(ctx, out, scraper.Error(scraper.ParseError(sceneURL, fmt.Errorf("no movie block on the page"))))
		return orEmpty(scene)
	}
	if d.title != "" {
		scene.Title = d.title
	}
	scene.Description = d.description
	scene.Performers = d.performers
	scene.Tags = d.tags
	if d.daysAgo > 0 {
		scene.Date = dateFromDaysAgo(now, d.daysAgo)
	}
	return scene
}

// dateFromDaysAgo turns the site's only date form into an absolute one. It is
// stable across runs — the site's own counter advances at the same rate — but
// is accurate only to the day, so the time of day is dropped.
func dateFromDaysAgo(now time.Time, days int) time.Time {
	t := now.AddDate(0, 0, -days)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func orEmpty(s models.Scene) models.Scene {
	if s.Title == "" {
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
	id      string
	title   string
	thumb   string
	daysAgo int
}

var (
	cardSplitRe = regexp.MustCompile(`<div class="movie-list__item">`)
	cardIDRe    = regexp.MustCompile(`href="[^"]*/member/movie/([0-9]+-[0-9]+)/index\.html"`)
	cardTitleRe = regexp.MustCompile(`(?s)<p class="mb-1 movie-list__title">\s*<a[^>]*>(.*?)</a>`)
	// The poster is behind a lazyload placeholder; the real path is on
	// `_data-src`, with the placeholder SVG on `src`.
	cardThumbRe = regexp.MustCompile(`_data-src="([^"]+)"`)
	daysAgoRe   = regexp.MustCompile(`(\d+)\s+days?\s+ago`)
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
			title: cleanText(firstSubmatch(cardTitleRe, chunk)),
			thumb: firstSubmatch(cardThumbRe, chunk),
		}
		if e.id == "" {
			continue
		}
		if v := firstSubmatch(daysAgoRe, chunk); v != "" {
			e.daysAgo, _ = strconv.Atoi(v)
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
	tags        []string
	daysAgo     int
}

var (
	scriptRe    = regexp.MustCompile(`(?s)<script.*?</script>`)
	styleRe     = regexp.MustCompile(`(?s)<style.*?</style>`)
	detailH1Re  = regexp.MustCompile(`(?s)<h1 class="h3[^"]*">(.*?)</h1>`)
	actorLinkRe = regexp.MustCompile(`(?s)<a[^>]+href="[^"]*/member/actor/[^"]*"[^>]*>(.*?)</a>`)
	// The cast is one <p class="mb-0"> immediately after the h1. Matching
	// actor links across the whole page instead swept in the site navigation
	// ("Models") and a related-performers rail, so a two-woman scene came back
	// with a dozen names.
	castParaRe = regexp.MustCompile(`(?s)<p class="mb-0">(.*?)</p>`)
	tagBlockRe = regexp.MustCompile(`(?s)<div class="search-tags[^"]*">(.*?)</div>`)
	anchorRe   = regexp.MustCompile(`(?s)<a[^>]*>(.*?)</a>`)
	// The synopsis is the paragraph between the cast block and the tag list.
	descRe = regexp.MustCompile(`(?s)</div>\s*<p>(.*?)</p>\s*<div class="search-tags`)
)

func parseDetail(body string) detail {
	body = styleRe.ReplaceAllString(scriptRe.ReplaceAllString(body, ""), "")

	d := detail{
		title:       cleanText(firstSubmatch(detailH1Re, body)),
		description: cleanText(firstSubmatch(descRe, body)),
	}
	if _, afterH1, ok := strings.Cut(body, "</h1>"); ok {
		for _, m := range actorLinkRe.FindAllStringSubmatch(firstSubmatch(castParaRe, afterH1), -1) {
			if n := cleanText(m[1]); n != "" {
				d.performers = appendUnique(d.performers, n)
			}
		}
	}
	for _, m := range anchorRe.FindAllStringSubmatch(firstSubmatch(tagBlockRe, body), -1) {
		if t := cleanText(m[1]); t != "" {
			d.tags = appendUnique(d.tags, t)
		}
	}
	if v := firstSubmatch(daysAgoRe, body); v != "" {
		d.daysAgo, _ = strconv.Atoi(v)
	}
	return d
}

// ---- helpers ----

var (
	tagStripRe = regexp.MustCompile(`<[^>]*>`)
	brRe       = regexp.MustCompile(`(?i)<br\s*/?>`)
)

func cleanText(s string) string {
	// Synopses are one paragraph of <br>-separated lines; the breaks become
	// spaces rather than running the sentences together.
	s = brRe.ReplaceAllString(s, " ")
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
	case strings.HasPrefix(ref, "http://"), strings.HasPrefix(ref, "https://"):
		return ref
	case strings.HasPrefix(ref, "/"):
		return base + ref
	default:
		return base + "/" + ref
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
