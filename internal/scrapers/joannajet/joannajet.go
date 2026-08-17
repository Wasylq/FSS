// Package joannajet scrapes joannajet.com.
//
// Two things shape this scraper.
//
// **The site is fetched over HTTP, deliberately.** Its HTTPS listener serves an
// incomplete certificate chain — the Network Solutions intermediate is missing,
// so verification fails with "unable to get local issuer certificate" and Go,
// which does not fetch intermediates via AIA, cannot complete the handshake.
// Plain HTTP answers 200. The scheme is forced rather than taken from the
// operator's URL, because `cmd.normalizeInputURL` upgrades a bare host to
// https and every such invocation would otherwise fail at the TLS layer.
//
// **There is no complete listing.** `home.php` shows sixteen scenes and ignores
// its own `offset` parameter; `movies_m.php` indexes 38 DVDs whose scenes union
// to 177, well short of the catalogue. What every scene has is a `vid`, and
// `scene_m.php?vid={n}` serves it, so the walk is an id sweep bounded by the
// highest vid those two pages reach.
package joannajet

import (
	"context"
	"fmt"
	"html"
	"net/http"
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
	siteID     = "joannajet"
	domain     = "joannajet.com"
	studioName = "Joanna Jet"

	// probeMargin is how far past the highest vid in reach the sweep
	// continues, so a scene published since the index pages were rendered is
	// still found.
	probeMargin = 30
	// maxProbeID bounds the sweep even if the discovered maximum is nonsense.
	// Vids reach the low 1100s today.
	maxProbeID = 6000
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
	matchRe   = regexp.MustCompile(`^https?://(?:w{2,4}\.)?joannajet\.com(?:/|$)`)
	sceneRe   = regexp.MustCompile(`scene_m\.php\?[^"'\s]*vid=(\d+)`)
	movieIDRe = regexp.MustCompile(`movies_m\.php\?action=display&(?:amp;)?movieid=(\d+)`)
)

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		domain,
		domain + "/home.php",
		domain + "/scene_m.php?vid={id}",
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

// base is always http for this host — see the package doc. baseOverride is the
// test server, which is http too.
func (s *Scraper) base() string {
	if s.baseOverride != "" {
		return strings.TrimSuffix(s.baseOverride, "/")
	}
	return "http://www." + domain
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	base := s.base()

	// A single scene URL is a legitimate thing to point at.
	if m := sceneRe.FindStringSubmatch(studioURL); m != nil {
		scraper.Debugf(1, "%s: scraping one scene %s", siteID, m[1])
		if !send(ctx, out, scraper.Progress(1)) {
			return
		}
		if sc, ok := s.fetchScene(ctx, studioURL, base, m[1], out); ok {
			send(ctx, out, scraper.Scene(sc))
		}
		return
	}

	highest, ok := s.highestVID(ctx, base, opts, out)
	if !ok {
		return
	}
	limit := highest + probeMargin
	if limit > maxProbeID {
		limit = maxProbeID
	}
	scraper.Debugf(1, "%s: sweeping vids 1..%d with %d workers", siteID, limit, opts.Workers)

	work := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < opts.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for vid := range work {
				if !sleep(ctx, opts.Delay) {
					return
				}
				sc, found := s.fetchScene(ctx, studioURL, base, strconv.Itoa(vid), out)
				if !found {
					continue
				}
				if !send(ctx, out, scraper.Scene(sc)) {
					return
				}
			}
		}()
	}

	for vid := 1; vid <= limit; vid++ {
		if opts.KnownIDs[strconv.Itoa(vid)] {
			// The sweep is by id, not by date, so a stored id says nothing
			// about the ids after it — it is skipped, never a stop.
			continue
		}
		select {
		case work <- vid:
		case <-ctx.Done():
			close(work)
			wg.Wait()
			return
		}
	}
	close(work)
	wg.Wait()
}

// highestVID learns the top of the id range from the home page and the DVD
// index — between them they name the newest scenes. It reports false when
// neither yielded a vid, which is a template change rather than an empty site.
func (s *Scraper) highestVID(ctx context.Context, base string, opts scraper.ListOpts, out chan<- scraper.SceneResult) (int, bool) {
	pages := []string{base + "/home.php", base + "/movies_m.php"}

	highest := 0
	var movieIDs []string
	for i, p := range pages {
		if ctx.Err() != nil {
			return 0, false
		}
		if i > 0 && !sleep(ctx, opts.Delay) {
			return 0, false
		}
		body, err := s.fetch(ctx, p)
		if err != nil {
			send(ctx, out, scraper.Error(fmt.Errorf("index %s: %w", p, err)))
			continue
		}
		highest = maxVID(highest, string(body))
		if strings.HasSuffix(p, "movies_m.php") {
			for _, m := range movieIDRe.FindAllStringSubmatch(string(body), -1) {
				movieIDs = append(movieIDs, m[1])
			}
		}
	}

	// The DVD pages carry the vids the index pages do not, and the newest
	// release is not always on the front page.
	for _, id := range movieIDs {
		if ctx.Err() != nil || !sleep(ctx, opts.Delay) {
			break
		}
		body, err := s.fetch(ctx, base+"/movies_m.php?action=display&movieid="+id)
		if err != nil {
			continue
		}
		highest = maxVID(highest, string(body))
	}

	if highest == 0 {
		send(ctx, out, scraper.Error(scraper.ParseError(base+"/home.php",
			fmt.Errorf("no scene links on the index pages — cannot bound the id sweep"))))
		return 0, false
	}
	scraper.Debugf(1, "%s: highest vid in reach is %d (across %d DVD pages)", siteID, highest, len(movieIDs))
	return highest, true
}

func maxVID(highest int, body string) int {
	for _, m := range sceneRe.FindAllStringSubmatch(body, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil && n > highest {
			highest = n
		}
	}
	return highest
}

// fetchScene reads one scene page. `found` is false for a vid the site does not
// have, which serves a page with an empty title rather than a 404 and is the
// ordinary outcome for part of the sweep.
func (s *Scraper) fetchScene(ctx context.Context, studioURL, base, vid string, out chan<- scraper.SceneResult) (models.Scene, bool) {
	sceneURL := base + "/scene_m.php?vid=" + vid

	body, err := s.fetch(ctx, sceneURL)
	if err != nil {
		send(ctx, out, scraper.Error(fmt.Errorf("scene %s: %w", vid, err)))
		return models.Scene{}, false
	}

	d := parseScene(string(body))
	if d.title == "" {
		return models.Scene{}, false
	}

	scene := models.Scene{
		ID:          vid,
		SiteID:      siteID,
		StudioURL:   studioURL,
		Title:       d.title,
		URL:         sceneURL,
		Studio:      studioName,
		Description: d.description,
		Date:        d.date,
		Resolution:  d.resolution,
		Thumbnail:   d.thumb,
		ScrapedAt:   time.Now().UTC(),
	}
	return scene, true
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

// ---- scene page ----

type sceneData struct {
	title       string
	description string
	date        time.Time
	resolution  string
	thumb       string
}

var (
	scriptRe = regexp.MustCompile(`(?s)<script.*?</script>`)
	styleRe  = regexp.MustCompile(`(?s)<style.*?</style>`)
	// The title is only in <title>, behind a fixed prefix.
	pageTitleRe = regexp.MustCompile(`(?s)<title>(.*?)</title>`)
	releasedRe  = regexp.MustCompile(`Released:\s*<strong>\s*([0-9]{1,2}\s+[A-Za-z]+\s+[0-9]{4})\s*</strong>`)
	qualityRe   = regexp.MustCompile(`Quality:\s*<strong>\s*([^<]+?)\s*</strong>`)
	descRe      = regexp.MustCompile(`(?s)<div class="JJVidDesc">(.*?)</div>`)
	thumbRe     = regexp.MustCompile(`<img[^>]+src="([^"]*/pics/gallery/[^"]+)"`)
	// Every scene page ends with a "More Videos" rail of other scenes, each
	// with its own title, date and quality. Cutting the page there is what
	// keeps a neighbour's fields out of this one.
	moreVideosRe = regexp.MustCompile(`(?i)<div class="moreHeader"|More Videos`)
)

// titlePrefixRe strips the fixed prefix every scene page's <title> carries.
// The separator is matched rather than trimmed as a literal because whitespace
// collapsing leaves a vid the site does not have reading exactly
// "Joanna Jet | Scene Preview -", which a literal trim does not empty — and it
// was then stored as a scene titled that.
var titlePrefixRe = regexp.MustCompile(`^Joanna Jet\s*\|\s*Scene Preview\s*-?\s*`)

func parseScene(body string) sceneData {
	body = styleRe.ReplaceAllString(scriptRe.ReplaceAllString(body, ""), "")

	title := cleanText(firstSubmatch(pageTitleRe, body))
	title = strings.TrimSpace(titlePrefixRe.ReplaceAllString(title, ""))
	if title == "" {
		// A vid the site does not have still renders the shell, with the
		// prefix and nothing after it.
		return sceneData{}
	}

	if loc := moreVideosRe.FindStringIndex(body); loc != nil {
		body = body[:loc[0]]
	}

	d := sceneData{
		title:       title,
		description: cleanText(firstSubmatch(descRe, body)),
		resolution:  cleanText(firstSubmatch(qualityRe, body)),
		thumb:       firstSubmatch(thumbRe, body),
	}
	if v := firstSubmatch(releasedRe, body); v != "" {
		if t, err := time.Parse("2 January 2006", strings.Join(strings.Fields(v), " ")); err == nil {
			d.date = t.UTC()
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
