package auntjudys

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/parseutil"
	"github.com/Wasylq/FSS/scraper"
)

const siteID = "auntjudys"

// listingPath is the only page the tour still serves scene data on. See
// docs/scrapers.md for what the 2026 redesign removed.
const listingPath = "/tour/categories/movies.html"

var (
	matchRe = regexp.MustCompile(`^https?://(?:www\.)?auntjudys(?:xxx)?\.com(?:/|$)`)
	baseRe  = regexp.MustCompile(`^(https?://(?:www\.)?auntjudys(?:xxx)?\.com)`)
)

type Scraper struct {
	client *http.Client
}

func New() *Scraper {
	return &Scraper{
		client: httpx.NewClient(30 * time.Second),
	}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }
func (s *Scraper) Patterns() []string {
	return []string{
		"auntjudysxxx.com",
		"auntjudysxxx.com/tour/categories/movies.html",
		"auntjudys.com",
		"auntjudys.com/tour/categories/movies.html",
	}
}
func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

func resolveBase(studioURL string) string {
	if m := baseRe.FindString(studioURL); m != "" {
		return strings.TrimRight(m, "/")
	}
	if idx := strings.Index(studioURL, "/tour/"); idx >= 0 {
		return studioURL[:idx]
	}
	return "https://www.auntjudysxxx.com"
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	base := resolveBase(studioURL)
	pageURL := base + listingPath
	scraper.Debugf(1, "auntjudys: fetching %s", pageURL)

	body, err := s.fetchPage(ctx, pageURL)
	if err != nil {
		select {
		case out <- scraper.Error(err):
		case <-ctx.Done():
		}
		return
	}

	scenes := parseListingPage(body, base, pageURL, time.Now().UTC())
	if len(scenes) == 0 {
		select {
		case out <- scraper.Error(scraper.ParseError(pageURL, fmt.Errorf("no update-item cards found"))):
		case <-ctx.Done():
		}
		return
	}
	scraper.Debugf(1, "auntjudys: %d scenes on the tour", len(scenes))

	select {
	case out <- scraper.Progress(len(scenes)):
	case <-ctx.Done():
		return
	}

	for i := range scenes {
		if opts.KnownIDs[scenes[i].ID] {
			scraper.Debugf(1, "auntjudys: hit known ID, stopping early")
			select {
			case out <- scraper.StoppedEarly():
			case <-ctx.Done():
			}
			return
		}
		scenes[i].StudioURL = studioURL
		select {
		case out <- scraper.Scene(scenes[i]):
		case <-ctx.Done():
			return
		}
	}
}

var (
	cardStartRe = regexp.MustCompile(`<div class="update-item`)
	cardIDRe    = regexp.MustCompile(`contentthumbs/[^"']*?/(\d+)-\d+x\.\w+`)
	cardThumbRe = regexp.MustCompile(`<img[^>]+src="([^"]+)"`)
	cardAltRe   = regexp.MustCompile(`<img[^>]+alt="([^"]*)"`)
	cardTitleRe = regexp.MustCompile(`(?s)class="update-title"[^>]*>(.*?)</a>`)
	cardDateRe  = regexp.MustCompile(`(?s)class="update-date">.*?(\d{2}/\d{2}/\d{4})`)
	cardTypeRe  = regexp.MustCompile(`(?s)class="update-type">(.*?)(?:<span class="update-date"|</div>)`)
	cardModelRe = regexp.MustCompile(`(?s)class="update_models">(.*?)</span>`)
	durationRe  = regexp.MustCompile(`^\d{1,2}:\d{2}(?::\d{2})?$`)
	tagStripRe  = regexp.MustCompile(`<[^>]+>`)
)

// parseListingPage reads the tour's update cards, deduped by set id. Photo
// sets are skipped: the grid mixes them with videos and only the update-type
// clock distinguishes the two.
func parseListingPage(body []byte, base, pageURL string, now time.Time) []models.Scene {
	locs := cardStartRe.FindAllIndex(body, -1)
	scenes := make([]models.Scene, 0, len(locs))
	seen := make(map[string]bool)

	for i, loc := range locs {
		end := len(body)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		block := body[loc[0]:end]

		idm := cardIDRe.FindSubmatch(block)
		if idm == nil || seen[string(idm[1])] {
			continue
		}
		id := string(idm[1])

		duration, isVideo := cardDuration(block)
		if !isVideo {
			continue
		}
		seen[id] = true

		scene := models.Scene{
			ID:        id,
			SiteID:    siteID,
			URL:       pageURL,
			Studio:    "Aunt Judy's",
			Duration:  duration,
			ScrapedAt: now,
		}
		if m := cardTitleRe.FindSubmatch(block); m != nil {
			scene.Title = cleanText(string(m[1]))
		}
		if scene.Title == "" {
			if m := cardAltRe.FindSubmatch(block); m != nil {
				scene.Title = cleanText(string(m[1]))
			}
		}
		if m := cardThumbRe.FindSubmatch(block); m != nil {
			scene.Thumbnail = absURL(base, string(m[1]))
		}
		if m := cardDateRe.FindSubmatch(block); m != nil {
			if t, err := time.Parse("01/02/2006", string(m[1])); err == nil {
				scene.Date = t.UTC()
			}
		}
		if m := cardModelRe.FindSubmatch(block); m != nil {
			scene.Performers = splitPerformers(string(m[1]))
		}
		if scene.Title == "" {
			continue
		}
		scenes = append(scenes, scene)
	}
	return scenes
}

// cardDuration reads the update-type badge, which holds either a clock
// ("20:41") for a video or a photo count ("191 Photos").
func cardDuration(block []byte) (int, bool) {
	m := cardTypeRe.FindSubmatch(block)
	if m == nil {
		return 0, false
	}
	text := cleanText(string(m[1]))
	if !durationRe.MatchString(text) {
		return 0, false
	}
	return parseutil.ParseDurationColon(text), true
}

func splitPerformers(s string) []string {
	var out []string
	for _, part := range strings.Split(cleanText(s), ",") {
		if name := strings.TrimSpace(part); name != "" {
			out = append(out, name)
		}
	}
	return out
}

func cleanText(s string) string {
	s = tagStripRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}

func absURL(base, ref string) string {
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

func (s *Scraper) fetchPage(ctx context.Context, rawURL string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     rawURL,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}
