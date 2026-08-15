// Package zone8 scrapes Zone 8 Media's two tours, englishlads.com and
// fityoungmen.com.
//
// Neither has a paginated listing worth walking: English Lads' updates are
// reachable only through its sitemap, and Fit Young Men's `/videos` page is a
// rotating selection. Both publish a complete `sitemap.xml`, and both encode
// the shoot id — and on English Lads the publication date — directly in the
// URL, so the sitemap alone gives an ordered work list before a single detail
// page is fetched.
//
// The two sites share a publisher and a URL convention but not a page
// template, so each has its own detail parser:
//
//   - English Lads: `/video-{YYYY-MM-DD}-{id}-{N|Y}-{slug}`, with an
//     `<h3 class="title">` naming the model, a `<p class="description">`, and a
//     `premium-descriptive-tags` list carrying the runtime and, on
//     pay-per-view updates, a price in GBP.
//   - Fit Young Men: `/{section}-{id}-{slug}`, with the model, sport and title
//     split across spans of one `<h1>`, a `Published: 9 Aug 26` line, and a
//     `model-profile-model-description` paragraph.
//
// Photo sets share the URL space on both sites and are excluded: English Lads
// prefixes them `photo-`, and on Fit Young Men only the sections listed in
// `sceneSections` are shoots rather than index pages.
package zone8

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Anastylosis/FSS/internal/httpx"
	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/scraper"
)

// Template selects the detail parser for a site.
type Template int

const (
	// TemplateEnglishLads is the `shoot-listing` page with an h3 title.
	TemplateEnglishLads Template = iota
	// TemplateFitYoungMen is the `model-profile` page with a split h1.
	TemplateFitYoungMen
)

// SiteConfig describes one tour.
type SiteConfig struct {
	SiteID     string
	Domain     string
	StudioName string
	Template   Template
	// Currency is the ISO code prices on this tour are quoted in; empty means
	// the tour quotes none, which is what gates price recording. It is not
	// stored with the snapshot — models.PriceSnapshot has no currency field —
	// so it exists to say out loud that English Lads' figures are pounds, not
	// the dollars every other priced scraper in FSS records.
	Currency string
}

var sites = []SiteConfig{
	{"englishlads", "englishlads.com", "English Lads", TemplateEnglishLads, "GBP"},
	{"fityoungmen", "fityoungmen.com", "Fit Young Men", TemplateFitYoungMen, "GBP"},
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

func (s *Scraper) ID() string { return s.cfg.SiteID }

func (s *Scraper) Patterns() []string {
	switch s.cfg.Template {
	case TemplateFitYoungMen:
		return []string{
			s.cfg.Domain,
			s.cfg.Domain + "/model-{name}-{id}-{slug}",
			s.cfg.Domain + "/nearly-nude-{id}-{slug}",
		}
	default:
		return []string{
			s.cfg.Domain,
			s.cfg.Domain + "/video-{date}-{id}-{flag}-{slug}",
		}
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
	return "https://www." + s.cfg.Domain
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	base := s.base(studioURL)

	// A single shoot URL is a legitimate thing to point at, and reading the
	// whole sitemap would be a strange way to honour it.
	if e, ok := s.entryFor(studioURL); ok {
		scraper.Debugf(1, "%s: scraping one shoot %s", s.cfg.SiteID, e.id)
		if !send(ctx, out, scraper.Progress(1)) {
			return
		}
		if sc := s.buildScene(ctx, studioURL, base, e, out); sc.ID != "" {
			send(ctx, out, scraper.Scene(sc))
		}
		return
	}

	entries, err := s.discover(ctx, base)
	if err != nil {
		send(ctx, out, scraper.Error(err))
		return
	}
	if len(entries) == 0 {
		send(ctx, out, scraper.Error(scraper.ParseError(base+"/sitemap.xml",
			fmt.Errorf("no shoot URLs in the sitemap"))))
		return
	}

	// The sitemap is not in a dependable order, and English Lads carries the
	// publication date in every URL, so sorting here is what makes the
	// KnownIDs stop below mean "everything older is already stored". Fit Young
	// Men has no date in its URLs; its ids are sequential and stand in.
	sortNewestFirst(entries)

	pending := entries
	stopped := false
	for i, e := range entries {
		if opts.KnownIDs[e.id] {
			scraper.Debugf(1, "%s: hit known ID %s, stopping early", s.cfg.SiteID, e.id)
			pending, stopped = entries[:i], true
			break
		}
	}
	if stopped && !send(ctx, out, scraper.StoppedEarly()) {
		return
	}
	if len(pending) == 0 {
		return
	}

	scraper.Debugf(1, "%s: %d shoots to fetch with %d workers", s.cfg.SiteID, len(pending), opts.Workers)
	if !send(ctx, out, scraper.Progress(len(pending))) {
		return
	}
	s.fetchAll(ctx, studioURL, base, pending, opts, out)
}

var locRe = regexp.MustCompile(`<loc>\s*([^<\s]+)\s*</loc>`)

func (s *Scraper) discover(ctx context.Context, base string) ([]listEntry, error) {
	body, err := s.fetch(ctx, base+"/sitemap.xml")
	if err != nil {
		return nil, fmt.Errorf("sitemap: %w", err)
	}
	seen := make(map[string]bool)
	var out []listEntry
	for _, m := range locRe.FindAllStringSubmatch(string(body), -1) {
		e, ok := s.entryFor(html.UnescapeString(m[1]))
		if !ok || seen[e.id] {
			continue
		}
		seen[e.id] = true
		out = append(out, e)
	}
	return out, nil
}

// ---- URL shapes ----

// englishLadsRe matches `/video-2025-05-16-02979-N-{slug}`. The `photo-` prefix
// on the same site is a photo set and is deliberately not matched.
var englishLadsRe = regexp.MustCompile(`^/video-(\d{4})-(\d{2})-(\d{2})-(\d+)-[A-Za-z]-(.+)$`)

// sceneSections are the Fit Young Men URL prefixes that name a shoot. Anything
// else on that host — `sport-rugby-player`, `all-models` — is an index page.
var sceneSections = []string{"model-", "nearly-nude-", "fit-and-famous-", "guest-photographer-"}

// fitYoungMenIDRe pulls the numeric shoot id out of a Fit Young Men path.
var fitYoungMenIDRe = regexp.MustCompile(`-(\d{3,})-`)

type listEntry struct {
	id   string
	path string
	date time.Time
}

// entryFor reports whether a URL names a shoot on this site, and describes it.
func (s *Scraper) entryFor(rawURL string) (listEntry, bool) {
	p := rawURL
	if u, err := url.Parse(rawURL); err == nil && u.EscapedPath() != "" {
		if u.Host != "" && !s.matchRe.MatchString(rawURL) {
			return listEntry{}, false
		}
		// EscapedPath, not Path: one Fit Young Men sitemap entry ends in a
		// literal `%0A`, and decoding it puts a newline in the path that then
		// makes the fetch URL unparseable. Keeping the escaped form leaves it
		// as the site published it.
		p = u.EscapedPath()
	}
	p = strings.TrimSpace(p)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	p = strings.TrimSuffix(p, "/")

	switch s.cfg.Template {
	case TemplateFitYoungMen:
		rest := strings.TrimPrefix(p, "/")
		matched := false
		for _, sec := range sceneSections {
			if strings.HasPrefix(rest, sec) {
				matched = true
				break
			}
		}
		if !matched {
			return listEntry{}, false
		}
		m := fitYoungMenIDRe.FindStringSubmatch(p)
		if m == nil {
			return listEntry{}, false
		}
		return listEntry{id: m[1], path: p}, true
	default:
		m := englishLadsRe.FindStringSubmatch(p)
		if m == nil {
			return listEntry{}, false
		}
		e := listEntry{id: m[4], path: p}
		if t, err := time.Parse("2006-01-02", m[1]+"-"+m[2]+"-"+m[3]); err == nil {
			e.date = t.UTC()
		}
		return e, true
	}
}

// sortNewestFirst orders shoots newest-first. English Lads carries the date in
// its URLs; Fit Young Men does not, so its sequential id stands in.
func sortNewestFirst(entries []listEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if !a.date.IsZero() || !b.date.IsZero() {
			if !a.date.Equal(b.date) {
				return a.date.After(b.date)
			}
		}
		ai, aerr := strconv.Atoi(a.id)
		bi, berr := strconv.Atoi(b.id)
		if aerr == nil && berr == nil {
			return ai > bi
		}
		return a.id > b.id
	})
}

func (s *Scraper) fetchAll(ctx context.Context, studioURL, base string, entries []listEntry, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
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
				sc := s.buildScene(ctx, studioURL, base, e, out)
				if sc.ID == "" {
					continue
				}
				if !send(ctx, out, scraper.Scene(sc)) {
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

// buildScene fetches one shoot page. A zero-value ID means the page could not
// be read and the failure has already been reported.
func (s *Scraper) buildScene(ctx context.Context, studioURL, base string, e listEntry, out chan<- scraper.SceneResult) models.Scene {
	sceneURL := base + e.path

	body, err := s.fetch(ctx, sceneURL)
	if err != nil {
		send(ctx, out, scraper.Error(fmt.Errorf("shoot %s: %w", e.id, err)))
		return models.Scene{}
	}

	var d detail
	if s.cfg.Template == TemplateFitYoungMen {
		d = parseFitYoungMen(string(body))
	} else {
		d = parseEnglishLads(string(body))
	}
	if d.title == "" {
		send(ctx, out, scraper.Error(scraper.ParseError(sceneURL, fmt.Errorf("no shoot block on the page"))))
		return models.Scene{}
	}

	scene := models.Scene{
		ID:          e.id,
		SiteID:      s.cfg.SiteID,
		StudioURL:   studioURL,
		Title:       d.title,
		URL:         sceneURL,
		Studio:      s.cfg.StudioName,
		Description: d.description,
		Performers:  d.performers,
		Duration:    d.duration,
		Date:        e.date,
		ScrapedAt:   time.Now().UTC(),
	}
	if !d.date.IsZero() {
		scene.Date = d.date
	}
	if d.thumb != "" {
		scene.Thumbnail = absolutize(d.thumb, base)
	}
	if d.preview != "" {
		scene.Preview = absolutize(d.preview, base)
	}
	scene.Tags = d.tags
	if d.sport != "" {
		scene.Categories = []string{d.sport}
	}
	// Only pay-per-view updates quote a price; membership-only ones quote
	// none, and recording a zero there would make every scene look free.
	// models.PriceSnapshot carries no currency, so the figure is stored bare —
	// see the Currency field's own note.
	if d.price > 0 && s.cfg.Currency != "" {
		scene.AddPrice(models.PriceSnapshot{
			Date:    scene.ScrapedAt,
			Regular: d.price,
		})
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

// ---- detail parsing ----

type detail struct {
	title       string
	description string
	performers  []string
	tags        []string
	duration    int
	price       float64
	thumb       string
	preview     string
	sport       string
	date        time.Time
}

var (
	scriptRe = regexp.MustCompile(`(?s)<script.*?</script>`)

	elTitleRe = regexp.MustCompile(`(?s)<h3 class="title">(.*?)</h3>`)
	// The older, non-premium layout: one h2 carrying "24th Feb 2006 - Title -
	// <a>Model</a>, <a>Model</a>". It covers 1462 of the 1508 updates; only the
	// recent pay-per-view ones use the h3 shape above.
	elH2Re      = regexp.MustCompile(`(?s)<h2 style="font-size: 13px;">(.*?)</h2>`)
	elH2DescRe  = regexp.MustCompile(`(?s)<div style="margin-top: 4px;[^"]*">(.*?)</div>`)
	elTagLinkRe = regexp.MustCompile(`(?s)<a[^>]+href="/\?tag=[^"]*"[^>]*>(.*?)</a>`)
	// "24th Feb 2006 - " prefixed to the title in the h2 shape.
	elH2DateRe = regexp.MustCompile(`^\s*(\d{1,2})(?:st|nd|rd|th)\s+([A-Za-z]{3,})\s+(\d{4})\s*-\s*`)
	// Attribute order differs between the two layouts — `<a href=… class=…>` on
	// the premium page, `<a class="more" href=…>` on the classic one — so the
	// href is matched wherever it falls.
	elModelRe   = regexp.MustCompile(`(?s)<a[^>]+href="/model-[^"]*"[^>]*>(.*?)</a>`)
	elDescRe    = regexp.MustCompile(`(?s)<p class="description">(.*?)</p>`)
	elTagsRe    = regexp.MustCompile(`(?s)<ul class="premium-descriptive-tags">(.*?)</ul>`)
	elMinutesRe = regexp.MustCompile(`(\d+)\s*minute`)
	elPriceRe   = regexp.MustCompile(`(?:&pound;|£)\s*([\d.]+)`)
	elPosterRe  = regexp.MustCompile(`data-poster="([^"]+)"`)
	elSourceRe  = regexp.MustCompile(`<source src="([^"]+\.mp4)"`)
	elImageRe   = regexp.MustCompile(`(?s)<div class="shoot-image">.*?<img[^>]+src="([^"]+)"`)

	fymH1Re     = regexp.MustCompile(`(?s)<h1>(.*?)</h1>`)
	fymNameRe   = regexp.MustCompile(`(?s)<span class="model-profile-name">(.*?)</span>`)
	fymSportRe  = regexp.MustCompile(`(?s)<span class="model-profile-sport">(.*?)</span>`)
	fymSiteRe   = regexp.MustCompile(`(?s)<span class="model-profile-site">.*?</span>`)
	fymNameSpan = regexp.MustCompile(`(?s)<span class="model-profile-name">.*?</span>`)
	fymPubRe    = regexp.MustCompile(`Published:\s*([0-9]{1,2}\s+[A-Za-z]{3}\s+[0-9]{2,4})`)
	fymDescRe   = regexp.MustCompile(`(?s)<div class="model-profile-model-description">(.*?)</div>`)
	fymThumbRe  = regexp.MustCompile(`<img[^>]+src="([^"]*mb\d+[^"]*\.jpg)"`)
	fymMinuteRe = regexp.MustCompile(`(\d+)\s*minute`)

	// The bonus-video pages (`nearly-nude-`, `fit-and-famous-`,
	// `guest-photographer-`) use a third layout with a `set-details` block
	// rather than the model-profile h1. A `model-` URL for one of them
	// redirects here, so this shape has to be handled or 69 shoots are lost.
	fymSetDetailsRe = regexp.MustCompile(`(?s)<div class="set-details">(.*?)<div class="highlight-indicators">`)
	fymSetTitleRe   = regexp.MustCompile(`(?s)<div class="title">(.*?)<div class="date`)
	fymSetSportRe   = regexp.MustCompile(`(?s)<div class="sport">(.*?)</div>`)
	fymSetDateRe    = regexp.MustCompile(`Published\s+([0-9]{1,2}\s+[A-Za-z]{3,}\s+[0-9]{2,4})`)
	fymSetDescRe    = regexp.MustCompile(`(?s)<div class="description">(.*?)</div>`)
	fymSetPosterRe  = regexp.MustCompile(`poster="([^"]+)"`)
	fymSetSourceRe  = regexp.MustCompile(`<source src="([^"]+\.mp4)"`)
	// The purchase boilerplate precedes the real synopsis, separated by a
	// double line break.
	fymBoilerplateRe = regexp.MustCompile(`(?s)^.*?<br\s*/?>\s*<br\s*/?>`)
)

func parseEnglishLads(body string) detail {
	body = scriptRe.ReplaceAllString(body, "")

	// Two layouts are in play. The recent pay-per-view updates use an
	// `<h3 class="title">` inside a `shoot-listing`; everything older — 1462 of
	// the 1508 updates — uses a single h2 that also carries the date, and is
	// the only one of the two with tags.
	if titleBlock := firstSubmatch(elTitleRe, body); titleBlock != "" {
		return parseEnglishLadsPremium(body, titleBlock)
	}
	return parseEnglishLadsClassic(body)
}

func parseEnglishLadsClassic(body string) detail {
	var d detail

	h2 := firstSubmatch(elH2Re, body)
	if h2 == "" {
		return d
	}
	d.performers = anchorTexts(elModelRe, h2)

	title := cleanText(elModelRe.ReplaceAllString(h2, ""))
	if m := elH2DateRe.FindStringSubmatch(title); m != nil {
		if t, ok := parseClassicDate(m[1] + " " + m[2] + " " + m[3]); ok {
			d.date = t
		}
		title = title[len(m[0]):]
	}
	d.title = strings.Trim(title, " ,-–—")

	d.description = cleanText(firstSubmatch(elH2DescRe, body))
	d.tags = anchorTexts(elTagLinkRe, body)
	d.preview = firstSubmatch(elSourceRe, body)
	d.thumb = firstSubmatch(elImageRe, body)
	if d.thumb == "" {
		d.thumb = firstSubmatch(elPosterRe, body)
	}
	return d
}

// parseClassicDate reads "24 Feb 2006" and "24 February 2006".
func parseClassicDate(s string) (time.Time, bool) {
	for _, layout := range []string{"2 Jan 2006", "2 January 2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

func parseEnglishLadsPremium(body, titleBlock string) detail {
	var d detail
	d.performers = anchorTexts(elModelRe, titleBlock)
	// The h3 is "Title - <a>Model</a>"; dropping the anchor leaves the title
	// plus the separator, which is trimmed off.
	d.title = strings.TrimRight(cleanText(elModelRe.ReplaceAllString(titleBlock, "")), " -–—")
	d.description = cleanText(firstSubmatch(elDescRe, body))

	tags := firstSubmatch(elTagsRe, body)
	if v := firstSubmatch(elMinutesRe, tags); v != "" {
		if mins, err := strconv.Atoi(v); err == nil {
			d.duration = mins * 60
		}
	}
	if v := firstSubmatch(elPriceRe, tags); v != "" {
		d.price, _ = strconv.ParseFloat(v, 64)
	}

	d.preview = firstSubmatch(elSourceRe, body)
	d.thumb = firstSubmatch(elImageRe, body)
	if d.thumb == "" {
		d.thumb = firstSubmatch(elPosterRe, body)
	}
	return d
}

func parseFitYoungMen(body string) detail {
	body = scriptRe.ReplaceAllString(body, "")
	if block := firstSubmatch(fymSetDetailsRe, body); block != "" {
		return parseFitYoungMenBonus(body, block)
	}
	return parseFitYoungMenProfile(body)
}

// parseFitYoungMenBonus reads the `set-details` layout used by the bonus-video
// sections. The title there is "Name - Title" with no markup separating them,
// so the model name is taken from the first segment.
func parseFitYoungMenBonus(body, block string) detail {
	var d detail

	title := cleanText(firstSubmatch(fymSetTitleRe, block))
	if title == "" {
		title = cleanText(firstSubmatch(regexp.MustCompile(`(?s)<div class="title">(.*?)</div>`), block))
	}
	if name, rest, ok := strings.Cut(title, " - "); ok {
		d.performers = []string{strings.TrimSpace(name)}
		d.title = strings.TrimSpace(rest)
	} else {
		d.title = title
	}

	d.sport = cleanText(firstSubmatch(fymSetSportRe, block))
	if v := firstSubmatch(fymSetDateRe, block); v != "" {
		if t, ok := parseFYMDate(v); ok {
			d.date = t
		}
	}

	desc := firstSubmatch(fymSetDescRe, block)
	if stripped := fymBoilerplateRe.ReplaceAllString(desc, ""); strings.TrimSpace(stripped) != "" {
		desc = stripped
	}
	d.description = cleanText(desc)

	d.thumb = firstSubmatch(fymSetPosterRe, body)
	d.preview = firstSubmatch(fymSetSourceRe, body)
	if v := firstSubmatch(elPriceRe, block); v != "" {
		d.price, _ = strconv.ParseFloat(v, 64)
	}
	return d
}

func parseFitYoungMenProfile(body string) detail {
	var d detail

	h1 := firstSubmatch(fymH1Re, body)
	name := cleanText(firstSubmatch(fymNameRe, h1))
	if name != "" {
		d.performers = []string{name}
	}
	// The h1 is four nested spans — site, name, sport and the title, with the
	// sport span *inside* the title span. Cutting the three known ones out and
	// keeping the remainder is steadier than trying to bound the title span,
	// which a stray `</span>` inside it defeats.
	d.sport = strings.TrimLeft(cleanText(firstSubmatch(fymSportRe, h1)), "- ")
	rest := fymSiteRe.ReplaceAllString(h1, "")
	rest = fymNameSpan.ReplaceAllString(rest, "")
	rest = fymSportRe.ReplaceAllString(rest, "")
	d.title = strings.Trim(cleanText(rest), " -–—")
	if d.title == "" {
		d.title = cleanText(h1)
	}

	d.description = cleanText(firstSubmatch(fymDescRe, body))
	d.thumb = firstSubmatch(fymThumbRe, body)
	if v := firstSubmatch(fymMinuteRe, body); v != "" {
		if mins, err := strconv.Atoi(v); err == nil {
			d.duration = mins * 60
		}
	}
	if v := firstSubmatch(fymPubRe, body); v != "" {
		if t, ok := parseFYMDate(v); ok {
			d.date = t
		}
	}
	return d
}

// parseFYMDate reads "9 Aug 26" and "9 Aug 2026". The two-digit form is this
// century — the site launched in 2006 and its own copy uses it throughout.
func parseFYMDate(s string) (time.Time, bool) {
	s = strings.Join(strings.Fields(s), " ")
	for _, layout := range []string{"2 Jan 2006", "2 Jan 06"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

// ---- helpers ----

var tagStripRe = regexp.MustCompile(`<[^>]*>`)

func cleanText(s string) string {
	s = tagStripRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, " ", " ")
	return strings.Join(strings.Fields(s), " ")
}

func anchorTexts(re *regexp.Regexp, s string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, m := range re.FindAllStringSubmatch(s, -1) {
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
	case strings.HasPrefix(ref, "http://"), strings.HasPrefix(ref, "https://"):
		return ref
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
