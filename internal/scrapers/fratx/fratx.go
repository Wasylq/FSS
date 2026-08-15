// Package fratx scrapes fratx.com.
//
// The tour has no all-videos listing: `sets.php` indexes models, and scenes are
// only reachable through `category.php?id={n}`. The union of every category the
// home page links reaches 345 scenes, and the 24 model sets add none the
// categories miss, so the category sweep is the catalogue.
//
// Listing cards carry the title, the date and a thumbnail. Each scene's own
// `trailer.php?id={n}` page adds the tags, the cast, the description and the
// player poster, so a detail fetch follows discovery.
package fratx

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Anastylosis/FSS/internal/httpx"
	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/scraper"
)

const (
	siteID     = "fratx"
	domain     = "fratx.com"
	studioName = "FratX"
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
	matchRe     = regexp.MustCompile(`^https?://(?:www\.)?fratx\.com(?:/|$)`)
	categoryURL = regexp.MustCompile(`category\.php\?id=(\d+)`)
	setsURL     = regexp.MustCompile(`sets\.php\?id=(\d+)`)
	trailerURL  = regexp.MustCompile(`trailer\.php\?id=(\d+)`)
)

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		domain,
		domain + "/category.php?id={id}",
		domain + "/sets.php?id={id}",
		domain + "/trailer.php?id={id}",
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
	return "https://" + domain
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	base := s.base(studioURL)

	// A single scene URL is a legitimate thing to point at.
	if m := trailerURL.FindStringSubmatch(studioURL); m != nil {
		scraper.Debugf(1, "%s: scraping one scene %s", siteID, m[1])
		if !send(ctx, out, scraper.Progress(1)) {
			return
		}
		if sc := s.buildScene(ctx, studioURL, base, listEntry{id: m[1]}, out); sc.ID != "" {
			send(ctx, out, scraper.Scene(sc))
		}
		return
	}

	var listings []string
	switch {
	case categoryURL.MatchString(studioURL):
		m := categoryURL.FindStringSubmatch(studioURL)
		scraper.Debugf(1, "%s: scraping category %s", siteID, m[1])
		listings = []string{"/category.php?id=" + m[1]}
	case setsURL.MatchString(studioURL):
		m := setsURL.FindStringSubmatch(studioURL)
		scraper.Debugf(1, "%s: scraping model set %s", siteID, m[1])
		listings = []string{"/sets.php?id=" + m[1]}
	default:
		var err error
		listings, err = s.categoryListings(ctx, base)
		if err != nil {
			send(ctx, out, scraper.Error(err))
			return
		}
		scraper.Debugf(1, "%s: %d categories to sweep", siteID, len(listings))
	}

	entries, ok := s.discover(ctx, base, listings, opts, out)
	if !ok || len(entries) == 0 {
		return
	}

	pending := entries
	stopped := false
	for i, e := range entries {
		if opts.KnownIDs[e.id] {
			// Scene ids rise with publication, and the sweep is sorted on them,
			// so a stored id means everything after it is stored too.
			scraper.Debugf(1, "%s: hit known ID %s, stopping early", siteID, e.id)
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

	scraper.Debugf(1, "%s: %d scenes to fetch with %d workers", siteID, len(pending), opts.Workers)
	if !send(ctx, out, scraper.Progress(len(pending))) {
		return
	}
	s.fetchAll(ctx, studioURL, base, pending, opts, out)
}

// categoryListings reads the category ids off the home page. They are the only
// route to the catalogue — there is no all-videos listing.
func (s *Scraper) categoryListings(ctx context.Context, base string) ([]string, error) {
	body, err := s.fetch(ctx, base+"/")
	if err != nil {
		return nil, fmt.Errorf("home page: %w", err)
	}
	seen := make(map[string]bool)
	var out []string
	for _, m := range categoryURL.FindAllStringSubmatch(string(body), -1) {
		if seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		out = append(out, "/category.php?id="+m[1])
	}
	if len(out) == 0 {
		return nil, scraper.ParseError(base+"/", fmt.Errorf("no categories on the home page"))
	}
	return out, nil
}

// maxListingPages bounds each category walk. Categories run to a handful of
// pages; the cap is a runaway guard.
const maxListingPages = 200

// discover walks every listing and returns the union, newest first. It reports
// false when the walk was cancelled.
func (s *Scraper) discover(ctx context.Context, base string, listings []string, opts scraper.ListOpts, out chan<- scraper.SceneResult) ([]listEntry, bool) {
	seen := make(map[string]bool)
	var entries []listEntry

	for _, listing := range listings {
		if ctx.Err() != nil {
			return entries, false
		}
		for page := 1; page <= maxListingPages; page++ {
			if !sleep(ctx, opts.Delay) {
				return entries, false
			}
			pageURL := fmt.Sprintf("%s%s&page=%d", base, listing, page)
			body, err := s.fetch(ctx, pageURL)
			if err != nil {
				// One unreachable category is not the catalogue. Report it and
				// keep sweeping: bailing here would hand an authoritative
				// --full save a catalogue missing everything after it.
				send(ctx, out, scraper.Error(fmt.Errorf("listing %s page %d: %w", listing, page, err)))
				break
			}
			cards := parseCards(string(body))
			if len(cards) == 0 {
				if page == 1 && len(listings) == 1 {
					// A named listing that parses to nothing is a template
					// change, not an empty category.
					send(ctx, out, scraper.Error(scraper.ParseError(pageURL, fmt.Errorf("no scene cards on the listing"))))
				}
				break
			}
			fresh := 0
			for _, c := range cards {
				if seen[c.id] {
					continue
				}
				seen[c.id] = true
				fresh++
				entries = append(entries, c)
			}
			// Past the last page the listing repeats, so a page that adds
			// nothing new is the end.
			if fresh == 0 {
				break
			}
		}
	}

	sortNewestFirst(entries)
	return entries, true
}

// sortNewestFirst orders the union by date, newest first, falling back to the
// numeric scene id. The sweep visits categories in home-page order, which is
// no order at all as far as publication goes.
func sortNewestFirst(entries []listEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if !a.date.Equal(b.date) {
			return a.date.After(b.date)
		}
		return numericID(a.id) > numericID(b.id)
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

// buildScene fetches the scene's own page. The card's title, date and thumbnail
// are the fallback for a detail that fails, so a failure costs the cast, tags
// and description rather than the scene. A single-scene run has no card, and
// there a failed detail yields nothing.
func (s *Scraper) buildScene(ctx context.Context, studioURL, base string, e listEntry, out chan<- scraper.SceneResult) models.Scene {
	sceneURL := base + "/trailer.php?id=" + e.id
	scene := models.Scene{
		ID:        e.id,
		SiteID:    siteID,
		StudioURL: studioURL,
		Title:     e.title,
		URL:       sceneURL,
		Studio:    studioName,
		Thumbnail: e.thumb,
		Date:      e.date,
		ScrapedAt: time.Now().UTC(),
	}

	body, err := s.fetch(ctx, sceneURL)
	if err != nil {
		send(ctx, out, scraper.Error(fmt.Errorf("scene %s: %w", e.id, err)))
		return orEmpty(scene)
	}
	d := parseDetail(string(body))
	if d.title == "" && len(d.performers) == 0 && d.description == "" {
		send(ctx, out, scraper.Error(scraper.ParseError(sceneURL, fmt.Errorf("no video info block on the page"))))
		return orEmpty(scene)
	}
	if d.title != "" {
		scene.Title = d.title
	}
	scene.Description = d.description
	scene.Performers = d.performers
	scene.Tags = d.tags
	if d.poster != "" {
		scene.Thumbnail = d.poster
	}
	if !d.date.IsZero() {
		scene.Date = d.date
	}
	return scene
}

// orEmpty drops a scene that has no title at all, which is what a single-scene
// run produces when its only fetch failed.
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
	id    string
	title string
	thumb string
	date  time.Time
}

var (
	cardSplitRe = regexp.MustCompile(`<div class="video-item">`)
	cardLinkRe  = regexp.MustCompile(`href="[^"]*trailer\.php\?id=(\d+)"`)
	cardThumbRe = regexp.MustCompile(`<img[^>]+src="([^"]+)"`)
	cardTitleRe = regexp.MustCompile(`(?s)<span class="video-title">(.*?)</span>`)
	cardDateRe  = regexp.MustCompile(`(?s)<span class="video-date">(.*?)</span>`)
)

func parseCards(body string) []listEntry {
	chunks := cardSplitRe.Split(body, -1)
	var out []listEntry
	for i, chunk := range chunks {
		if i == 0 {
			continue // preamble before the first card
		}
		m := cardLinkRe.FindStringSubmatch(chunk)
		if m == nil {
			continue
		}
		e := listEntry{
			id:    m[1],
			title: cleanText(firstSubmatch(cardTitleRe, chunk)),
			thumb: firstSubmatch(cardThumbRe, chunk),
		}
		if d, ok := parseDate(cleanText(firstSubmatch(cardDateRe, chunk))); ok {
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
	performers  []string
	tags        []string
	poster      string
	date        time.Time
}

var (
	scriptRe   = regexp.MustCompile(`(?s)<script.*?</script>`)
	infoWrapRe = regexp.MustCompile(`(?s)<div class="VideoInfoWrap">(.*?)<!-- END VideoInfoWrap -->`)
	tagTextRe  = regexp.MustCompile(`(?s)<span class="tag-text">(.*?)</span>`)
	modelsRe   = regexp.MustCompile(`(?s)<ul class="ModelNames">(.*?)</ul>`)
	anchorRe   = regexp.MustCompile(`(?s)<a[^>]*>(.*?)</a>`)
	nameRe     = regexp.MustCompile(`(?s)<div class="name">\s*<span>(.*?)</span>`)
	descRe     = regexp.MustCompile(`(?s)<div class="VideoDescription">(.*?)</div>`)
	posterRe   = regexp.MustCompile(`poster="([^"]+)"`)
	// The description opens with the publication date: "September 10th, 2025 - ".
	descDateRe = regexp.MustCompile(`^\s*([A-Z][a-z]+\s+\d{1,2})(?:st|nd|rd|th)?,\s*(\d{4})\s*-\s*`)
)

func parseDetail(body string) detail {
	body = scriptRe.ReplaceAllString(body, "")
	var d detail

	// Everything is scoped to VideoInfoWrap: the page also carries a
	// "Related Videos" grid whose cards would otherwise supply titles and
	// dates, and a footer full of anchors.
	info := firstSubmatch(infoWrapRe, body)
	if info == "" {
		return d
	}

	d.title = cleanText(firstSubmatch(nameRe, info))
	for _, m := range tagTextRe.FindAllStringSubmatch(info, -1) {
		if t := cleanText(m[1]); t != "" {
			d.tags = appendUnique(d.tags, t)
		}
	}
	for _, m := range anchorRe.FindAllStringSubmatch(firstSubmatch(modelsRe, info), -1) {
		if n := cleanText(m[1]); n != "" {
			d.performers = appendUnique(d.performers, n)
		}
	}

	desc := cleanText(firstSubmatch(descRe, info))
	if m := descDateRe.FindStringSubmatch(desc); m != nil {
		if t, ok := parseDate(m[1] + ", " + m[2]); ok {
			d.date = t
		}
		desc = strings.TrimSpace(desc[len(m[0]):])
	}
	d.description = desc

	d.poster = firstSubmatch(posterRe, body)
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
	for _, layout := range []string{"Jan 2, 2006", "January 2, 2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

func numericID(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return n
		}
		n = n*10 + int(r-'0')
	}
	return n
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
