// Package shinybound scrapes ShinyBound Productions' two tours,
// shinybound.com and shinysboundsluts.com, which run the same PaySiteManager
// CMS.
//
// The `/videos?page={N}` listing is server-rendered and its cards are almost
// the whole record: scene id, title, detail URL, thumbnail, performer, runtime,
// photo count, date, and a per-scene price. Each `/updates/{slug}` page adds
// the description and the full tag list, so discovery is followed by a detail
// fetch.
//
// Every detail page also carries a "related videos" rail built from the very
// same card markup, so all detail parsing is scoped to the block *before* it.
package shinybound

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
	"github.com/Anastylosis/FSS/parseutil"
	"github.com/Anastylosis/FSS/scraper"
)

// RecommendedDelay is the delay at which these tours stay reachable. They
// rate-limit hard: `--delay 400 -w 4` drew 30 HTTP 429s and stopped the walk at
// 216 of ~1638 scenes, and a faster run locked the whole host out for minutes.
// `--delay 1500 -w 2` completed 456 scenes with none. It is **not** silently
// enforced — the operator's `opts.Delay` is always honoured, and
// `WarnDelayBelow` surfaces a one-shot warning below it.
const RecommendedDelay = 1500 * time.Millisecond

// SiteConfig describes one tour on the CMS.
type SiteConfig struct {
	SiteID     string
	Domain     string
	StudioName string
}

var sites = []SiteConfig{
	{"shinybound", "shinybound.com", "ShinyBound"},
	{"shinysboundsluts", "shinysboundsluts.com", "ShinysBoundSluTS"},
}

type Scraper struct {
	Client       *http.Client
	cfg          SiteConfig
	matchRe      *regexp.Regexp
	baseOverride string
}

func New(cfg SiteConfig) *Scraper {
	return &Scraper{
		Client:  httpx.NewClient(30 * time.Second),
		cfg:     cfg,
		matchRe: regexp.MustCompile(`^https?://(?:www\.)?` + regexp.QuoteMeta(cfg.Domain) + `(?:/|$)`),
	}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() {
	for _, cfg := range sites {
		scraper.Register(New(cfg))
	}
}

var sceneRe = regexp.MustCompile(`/updates/([^/?#]+)`)

func (s *Scraper) ID() string { return s.cfg.SiteID }

func (s *Scraper) Patterns() []string {
	return []string{
		s.cfg.Domain,
		s.cfg.Domain + "/videos",
		s.cfg.Domain + "/updates/{slug}",
	}
}

func (s *Scraper) MatchesURL(u string) bool { return s.matchRe.MatchString(u) }

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
	return "https://" + s.cfg.Domain
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	scraper.WarnDelayBelow(s.cfg.SiteID, opts.Delay, RecommendedDelay)

	base := s.base(studioURL)

	// A single scene URL is a legitimate thing to point at.
	if m := sceneRe.FindStringSubmatch(studioURL); m != nil {
		scraper.Debugf(1, "%s: scraping one scene %s", s.cfg.SiteID, m[1])
		if !send(ctx, out, scraper.Progress(1)) {
			return
		}
		if sc := s.buildScene(ctx, studioURL, base, listEntry{slug: m[1]}, out); sc.ID != "" {
			send(ctx, out, scraper.Scene(sc))
		}
		return
	}

	scraper.Debugf(1, "%s: walking the videos listing", s.cfg.SiteID)

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

// maxListingPages bounds the walk; the catalogues run to a few dozen pages.
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

		pageURL := fmt.Sprintf("%s/videos?page=%d", base, page)
		scraper.Debugf(1, "%s: fetching listing page %d", s.cfg.SiteID, page)
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
				// The listing is newest-first, so a stored id means the rest
				// of the walk is stored too.
				scraper.Debugf(1, "%s: hit known ID %s, stopping early", s.cfg.SiteID, c.id)
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

// buildScene fetches the scene's own page for the description and tags. The
// card's fields are the fallback for a detail that fails — it already carries
// id, title, date, runtime, performer, thumbnail and price.
func (s *Scraper) buildScene(ctx context.Context, studioURL, base string, e listEntry, out chan<- scraper.SceneResult) models.Scene {
	sceneURL := base + "/updates/" + e.slug
	scene := models.Scene{
		ID:        e.id,
		SiteID:    s.cfg.SiteID,
		StudioURL: studioURL,
		Title:     e.title,
		URL:       sceneURL,
		Studio:    s.cfg.StudioName,
		Thumbnail: e.thumb,
		Date:      e.date,
		Duration:  e.duration,
		ScrapedAt: time.Now().UTC(),
	}
	if e.performer != "" {
		scene.Performers = []string{e.performer}
	}
	if e.price > 0 {
		scene.AddPrice(models.PriceSnapshot{Date: scene.ScrapedAt, Regular: e.price})
	}

	body, err := s.fetch(ctx, sceneURL)
	if err != nil {
		send(ctx, out, scraper.Error(fmt.Errorf("scene %s: %w", e.slug, err)))
		return orEmpty(scene)
	}
	d := parseDetail(string(body))
	if d.title == "" && d.description == "" && len(d.tags) == 0 {
		send(ctx, out, scraper.Error(scraper.ParseError(sceneURL, fmt.Errorf("no scene block on the page"))))
		return orEmpty(scene)
	}
	if d.title != "" {
		scene.Title = d.title
	}
	scene.Description = d.description
	scene.Tags = d.tags
	if len(d.performers) > 0 {
		scene.Performers = d.performers
	}
	if scene.ID == "" {
		scene.ID = e.slug
	}
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
	id        string
	slug      string
	title     string
	thumb     string
	performer string
	date      time.Time
	duration  int
	price     float64
}

var (
	cardSplitRe = regexp.MustCompile(`<div class="\s*videoBlock">`)
	cardSlugRe  = regexp.MustCompile(`href="[^"]*/updates/([^"/?#]+)"`)
	cardIDRe    = regexp.MustCompile(`<img id="(\d+)"`)
	cardThumbRe = regexp.MustCompile(`<img[^>]+src="([^"]+)"`)
	cardTitleRe = regexp.MustCompile(`(?s)<h3>\s*<a[^>]*>(.*?)</a>`)
	cardModelRe = regexp.MustCompile(`(?s)<div class="modelName">\s*<a[^>]*>(.*?)</a>`)
	cardClockRe = regexp.MustCompile(`fa-clock"></i>\s*(\d{1,2}:\d{2}(?::\d{2})?)`)
	cardDateRe  = regexp.MustCompile(`fa-calendar"></i>\s*([A-Z][a-z]{2}\s+\d{1,2},\s*\d{4})`)
	cardPriceRe = regexp.MustCompile(`data-price="\$?([\d.]+)"`)
)

func parseCards(body string) []listEntry {
	chunks := cardSplitRe.Split(body, -1)
	var out []listEntry
	for i, chunk := range chunks {
		if i == 0 {
			continue // preamble before the first card
		}
		e := listEntry{
			id:        firstSubmatch(cardIDRe, chunk),
			slug:      firstSubmatch(cardSlugRe, chunk),
			title:     cleanText(firstSubmatch(cardTitleRe, chunk)),
			thumb:     firstSubmatch(cardThumbRe, chunk),
			performer: cleanText(firstSubmatch(cardModelRe, chunk)),
		}
		if e.slug == "" {
			continue
		}
		if e.id == "" {
			// Only the listing carries the numeric id; a related-rail card
			// does not. The slug is the fallback key.
			e.id = e.slug
		}
		if v := firstSubmatch(cardClockRe, chunk); v != "" {
			e.duration = parseutil.ParseDurationColon(v)
		}
		if v := firstSubmatch(cardDateRe, chunk); v != "" {
			if t, ok := parseDate(v); ok {
				e.date = t
			}
		}
		if v := firstSubmatch(cardPriceRe, chunk); v != "" {
			e.price, _ = strconv.ParseFloat(v, 64)
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
}

var (
	scriptRe = regexp.MustCompile(`(?s)<script.*?</script>`)
	// Every detail page ends with a "related videos" rail built from the same
	// card markup as the listing. Cutting the page there is what keeps another
	// scene's title, cast and tags out of this one.
	relatedRe   = regexp.MustCompile(`(?i)<div class="relatedVideos"|<div class="subTitle">\s*related videos`)
	detailH1Re  = regexp.MustCompile(`(?s)<h1[^>]*>(.*?)</h1>`)
	descBlockRe = regexp.MustCompile(`(?s)<div class="description">(.*?)</div>`)
	modelsRe    = regexp.MustCompile(`(?s)<div class="models">(.*?)</div>`)
	tagsRe      = regexp.MustCompile(`(?s)<div class="tags">(.*?)</div>`)
	anchorRe    = regexp.MustCompile(`(?s)<a[^>]*>(.*?)</a>`)
	paraRe      = regexp.MustCompile(`(?s)<p>(.*?)</p>`)
)

func parseDetail(body string) detail {
	body = scriptRe.ReplaceAllString(body, "")
	if loc := relatedRe.FindStringIndex(body); loc != nil {
		body = body[:loc[0]]
	}

	d := detail{title: cleanText(firstSubmatch(detailH1Re, body))}
	for _, m := range anchorRe.FindAllStringSubmatch(firstSubmatch(modelsRe, body), -1) {
		if n := cleanText(m[1]); n != "" {
			d.performers = appendUnique(d.performers, n)
		}
	}
	for _, m := range anchorRe.FindAllStringSubmatch(firstSubmatch(tagsRe, body), -1) {
		if t := cleanText(m[1]); t != "" {
			d.tags = appendUnique(d.tags, t)
		}
	}

	if block := firstSubmatch(descBlockRe, body); block != "" {
		d.description = cleanText(block)
	}
	if d.description == "" {
		// The synopsis is a bare paragraph on some builds; take the longest
		// one, since the shorter ones are navigation and legal boilerplate.
		best := ""
		for _, m := range paraRe.FindAllStringSubmatch(body, -1) {
			if t := cleanText(m[1]); len(t) > len(best) {
				best = t
			}
		}
		d.description = best
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

func parseDate(s string) (time.Time, bool) {
	s = strings.Join(strings.Fields(s), " ")
	for _, layout := range []string{"Jan 2, 2006", "Jan 02, 2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
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
