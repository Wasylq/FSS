// Package hotoldermale scrapes hotoldermale.com, Pantheon Productions' tour.
//
// The `/scenes?page={N}` listing is server-rendered — 24 cards a page, 36 pages,
// 847 scenes — but **past the last page it wraps back to page 1** rather than
// emptying or 404ing, so the walk stops on a page that adds no new id. Cards
// carry id, title, thumbnail, date, runtime and a truncated credit list
// ("… and 5 more"), so each scene's own page is fetched for the full cast, the
// description and the categories.
//
// `/profile/{id}-{slug}` and `/scenes/category/{id}-{slug}` render the same
// cards, so a URL naming either scrapes that slice through the same walk.
package hotoldermale

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
	siteID     = "hotoldermale"
	domain     = "hotoldermale.com"
	studioName = "Hot Older Male"
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
	matchRe    = regexp.MustCompile(`^https?://(?:www\.)?hotoldermale\.com(?:/|$)`)
	profileRe  = regexp.MustCompile(`/profile/(\d+-[^/?#]+)`)
	categoryRe = regexp.MustCompile(`/scenes/category/(\d+-[^/?#]+)`)
	sceneRe    = regexp.MustCompile(`/scene/(\d+)-([^/?#]+)`)
)

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		domain,
		domain + "/scenes",
		domain + "/scenes/category/{id}-{slug}",
		domain + "/profile/{id}-{slug}",
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

// listingPath is the path a walk pages through, without the page parameter.
func listingPath(studioURL string) string {
	if m := profileRe.FindStringSubmatch(studioURL); m != nil {
		return "/profile/" + m[1]
	}
	if m := categoryRe.FindStringSubmatch(studioURL); m != nil {
		return "/scenes/category/" + m[1]
	}
	return "/scenes"
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	base := s.base(studioURL)

	// A single scene URL is a legitimate thing to point at, and walking the
	// whole catalogue would be a strange way to honour it.
	if m := sceneRe.FindStringSubmatch(studioURL); m != nil {
		scraper.Debugf(1, "%s: scraping one scene %s", siteID, m[1])
		if !send(ctx, out, scraper.Progress(1)) {
			return
		}
		sc := s.buildScene(ctx, studioURL, base, listEntry{id: m[1], path: "/scene/" + m[1] + "-" + m[2]}, out)
		if sc.ID != "" {
			send(ctx, out, scraper.Scene(sc))
		}
		return
	}

	path := listingPath(studioURL)
	scraper.Debugf(1, "%s: walking %s", siteID, path)

	// Discovery and detail fetching run together. Walking all 36 listing pages
	// before the first detail put the first scene 43s away; streaming each
	// card into the pool as it is seen makes that the first two requests. The
	// count is therefore only known at the end, so the progress total is sent
	// then — the cmd layer shows a running count until it arrives.
	work := make(chan listEntry)
	var wg sync.WaitGroup
	for i := 0; i < opts.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.detailWorker(ctx, studioURL, base, work, opts, out)
		}()
	}

	found := s.discover(ctx, base, path, opts, out, work)
	close(work)
	wg.Wait()

	if found > 0 {
		send(ctx, out, scraper.Progress(found))
	}
}

// maxListingPages bounds the walk. The catalogue is 36 pages; the cap matters
// because the listing wraps rather than ending, so the only real terminator is
// the no-new-ids test below.
const maxListingPages = 500

// discover walks the listing, handing each new scene to the detail pool as it
// is seen. It returns how many it sent.
func (s *Scraper) discover(ctx context.Context, base, path string, opts scraper.ListOpts, out chan<- scraper.SceneResult, work chan<- listEntry) int {
	seen := make(map[string]bool)
	sent := 0

	for page := 1; page <= maxListingPages; page++ {
		if ctx.Err() != nil {
			return sent
		}
		if page > 1 && !sleep(ctx, opts.Delay) {
			return sent
		}

		pageURL := fmt.Sprintf("%s%s?page=%d", base, path, page)
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
				// The listing is newest-first, so a stored id means the rest
				// of the walk is stored too.
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
		// Past the last page the site re-serves page 1, so a page that adds
		// nothing new is the end. An empty-page test would loop to the cap.
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

// buildScene fetches the scene's own page. The card's fields are the fallback
// for a detail page that fails or changes shape — it already carries id, title,
// date, runtime, thumbnail and a truncated credit list, so a failure costs the
// description and the full cast rather than the scene.
func (s *Scraper) buildScene(ctx context.Context, studioURL, base string, e listEntry, out chan<- scraper.SceneResult) models.Scene {
	sceneURL := base + e.path
	scene := models.Scene{
		ID:         e.id,
		SiteID:     siteID,
		StudioURL:  studioURL,
		Title:      e.title,
		URL:        sceneURL,
		Studio:     studioName,
		Thumbnail:  e.thumb,
		Date:       e.date,
		Duration:   e.duration,
		Performers: e.performers,
		Likes:      e.likes,
		ScrapedAt:  time.Now().UTC(),
	}

	body, err := s.fetch(ctx, sceneURL)
	if err != nil {
		send(ctx, out, scraper.Error(fmt.Errorf("scene %s: %w", e.id, err)))
		return scene
	}
	d := parseDetail(string(body))
	if d.title == "" && d.description == "" && len(d.performers) == 0 {
		send(ctx, out, scraper.Error(scraper.ParseError(sceneURL, fmt.Errorf("no scene block on the detail page"))))
		return scene
	}
	if d.title != "" {
		scene.Title = d.title
	}
	scene.Description = d.description
	scene.Categories = d.categories
	if len(d.performers) > 0 {
		scene.Performers = d.performers
	}
	if !d.date.IsZero() {
		scene.Date = d.date
	}
	if d.duration > 0 {
		scene.Duration = d.duration
	}
	if d.thumb != "" {
		scene.Thumbnail = d.thumb
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
	thumb      string
	date       time.Time
	duration   int
	performers []string
	likes      int
}

var (
	cardSplitRe    = regexp.MustCompile(`<div class="[^"]*scene_container[^"]*"`)
	cardSceneRe    = regexp.MustCompile(`href="(/scene/(\d+)-[^"?#]*)"`)
	cardTitleRe    = regexp.MustCompile(`(?s)<div class="wrapperSceneTitle">\s*<a[^>]*>(.*?)</a>`)
	cardThumbRe    = regexp.MustCompile(`<img[^>]+src="([^"]+)"`)
	cardMinutesRe  = regexp.MustCompile(`<i class="icon-clock-1"></i>\s*(\d+)\s*min`)
	cardDateRe     = regexp.MustCompile(`(?s)<span class="dateLbl">(.*?)</span>`)
	cardLikesRe    = regexp.MustCompile(`(?s)class="[^"]*likesLbl[^"]*"[^>]*>\s*(\d+)\s*<`)
	profileLinkRe  = regexp.MustCompile(`(?s)<a href="/profile/\d+-[^"]*"[^>]*>(.*?)</a>`)
	andNMoreSuffix = regexp.MustCompile(`(?i)\s*and\s+\d+\s+more\.?\s*$`)
)

func parseCards(body string) []listEntry {
	chunks := cardSplitRe.Split(body, -1)
	var out []listEntry
	for i, chunk := range chunks {
		if i == 0 {
			continue // preamble before the first card
		}
		m := cardSceneRe.FindStringSubmatch(chunk)
		if m == nil {
			continue
		}
		e := listEntry{
			id:         m[2],
			path:       m[1],
			title:      cleanText(firstSubmatch(cardTitleRe, chunk)),
			thumb:      firstSubmatch(cardThumbRe, chunk),
			performers: anchorTexts(profileLinkRe, chunk),
		}
		if v := firstSubmatch(cardMinutesRe, chunk); v != "" {
			if mins, err := strconv.Atoi(v); err == nil {
				e.duration = mins * 60
			}
		}
		if d, ok := parseDate(cleanText(firstSubmatch(cardDateRe, chunk))); ok {
			e.date = d
		}
		if v := firstSubmatch(cardLikesRe, chunk); v != "" {
			e.likes, _ = strconv.Atoi(v)
		}
		out = append(out, e)
	}
	return out
}

// ---- detail page ----

type detail struct {
	title       string
	description string
	thumb       string
	date        time.Time
	duration    int
	performers  []string
	categories  []string
}

var (
	detailTitleRe = regexp.MustCompile(`(?s)<h2 class="[^"]*sectionMainTitle[^"]*">(.*?)</h2>`)
	// The description is the paragraph immediately before the Categories line;
	// anchoring on that heading is what keeps the join-form copy out.
	detailDescRe   = regexp.MustCompile(`(?s)<p>(.*?)</p>\s*<h5 class="strong">Categories:`)
	detailCatsRe   = regexp.MustCompile(`(?s)<h5 class="strong">Categories:(.*?)</h5>`)
	detailDetailRe = regexp.MustCompile(`(?s)<h5 class="strong">Details:(.*?)</h5>`)
	detailPerfRe   = regexp.MustCompile(`(?s)<span class="perfImage"\s*>(.*?)</span>`)
	categoryLinkRe = regexp.MustCompile(`(?s)<a href="/scenes/category/[^"]*"[^>]*>(.*?)</a>`)
	ogImageRe      = regexp.MustCompile(`<meta property="og:image" content="([^"]+)"`)
	// og:image is missing on a good share of the catalogue (125 of 847), so the
	// first `scn_` still on the page is the fallback — those are the scene's
	// own promo frames and are named after its id.
	sceneThumbRe = regexp.MustCompile(`src="([^"]*/_thumbs/[^"]*scn_\d+_[^"]+)"`)
	minutesRe    = regexp.MustCompile(`(\d+)\s*min`)
	dateInTextRe = regexp.MustCompile(`([A-Z][a-z]{2}\s+\d{1,2},\s+\d{4})`)
)

func parseDetail(body string) detail {
	var d detail
	d.title = cleanText(firstSubmatch(detailTitleRe, body))
	d.description = cleanText(firstSubmatch(detailDescRe, body))
	d.thumb = firstSubmatch(ogImageRe, body)
	if d.thumb == "" {
		d.thumb = firstSubmatch(sceneThumbRe, body)
	}
	d.categories = anchorTexts(categoryLinkRe, firstSubmatch(detailCatsRe, body))

	// The cast strip below the player is the full list; the line under the
	// title is truncated to the first few names on multi-performer scenes.
	for _, m := range detailPerfRe.FindAllStringSubmatch(body, -1) {
		for _, n := range anchorTexts(profileLinkRe, m[1]) {
			d.performers = appendUnique(d.performers, n)
		}
	}

	details := firstSubmatch(detailDetailRe, body)
	if v := firstSubmatch(minutesRe, details); v != "" {
		if mins, err := strconv.Atoi(v); err == nil {
			d.duration = mins * 60
		}
	}
	if t, ok := parseDate(firstSubmatch(dateInTextRe, cleanText(details))); ok {
		d.date = t
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

func anchorTexts(re *regexp.Regexp, s string) []string {
	var out []string
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		// Card credit lines end "…, Dallas Steele and 5 more." — the trailing
		// phrase is inside the last anchor on some cards and outside it on
		// others, so it is stripped either way.
		t := cleanText(andNMoreSuffix.ReplaceAllString(m[1], ""))
		if t == "" {
			continue
		}
		out = appendUnique(out, t)
	}
	return out
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
	for _, layout := range []string{"Jan 2, 2006", "January 2, 2006"} {
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
