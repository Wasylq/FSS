// Package alsangels scrapes scene metadata from alsangels.com.
//
// The entire video catalogue lives on a single static page
// (https://alsangels.com/dailyvideos.html, ~1700 <tr> blocks), so this scraper
// fetches that one page, parses every video block, and emits each as a scene.
// There is no pagination. An age-gate cookie (age_verified=true) is required on
// every request or the site redirects to /age-gate.html.
package alsangels

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

func init() { scraper.Register(New()) }

const (
	siteID      = "alsangels"
	studioName  = "ALS Angels"
	defaultBase = "https://alsangels.com"
	videosPath  = "/dailyvideos.html"
)

// dateLayout matches the site's "June 18, 2026" format.
const dateLayout = "January 02, 2006"

type Scraper struct {
	client *http.Client
	base   string // overridable in tests
}

func New() *Scraper {
	return &Scraper{
		client: httpx.NewClient(60 * time.Second),
		base:   defaultBase,
	}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		"alsangels.com",
		"alsangels.com/dailyvideos.html",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?alsangels\.com(?:/|$)`)

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// ---- runner ----

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	pageURL := s.base + videosPath
	scraper.Debugf(1, "alsangels: fetching catalogue %s", pageURL)

	body, err := s.fetchPage(ctx, pageURL)
	if err != nil {
		select {
		case out <- scraper.Error(fmt.Errorf("fetch catalogue: %w", err)):
		case <-ctx.Done():
		}
		return
	}

	items := parseItems(body)
	scraper.Debugf(1, "alsangels: parsed %d video blocks", len(items))

	if len(items) > 0 {
		select {
		case out <- scraper.Progress(len(items)):
		case <-ctx.Done():
			return
		}
	}

	now := time.Now().UTC()
	for _, it := range items {
		if opts.KnownIDs[it.id] {
			scraper.Debugf(1, "alsangels: hit known ID %s, stopping early", it.id)
			select {
			case out <- scraper.StoppedEarly():
			case <-ctx.Done():
			}
			return
		}
		select {
		case out <- scraper.Scene(s.toScene(it, studioURL, now)):
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scraper) fetchPage(ctx context.Context, rawURL string) ([]byte, error) {
	headers := httpx.BrowserHeaders(httpx.UserAgentFirefox)
	headers["Cookie"] = "age_verified=true"

	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     rawURL,
		Headers: headers,
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}
	return body, nil
}

// ---- parsing ----

var (
	// blockRe isolates each live video <tr> block. Upcoming/preview rows above
	// the "Upcoming" marker are HTML-commented out, so they never match here
	// (FindAll only sees uncommented <tr>...</tr> spans because commented rows
	// are wrapped in <!-- ... -->; we additionally guard on the marker below).
	upcomingMarkerRe = regexp.MustCompile(`(?s)\^\^\^\^\^\^ Upcoming \^\^\^\^\^\^`)
	blockRe          = regexp.MustCompile(`(?s)<tr>(.*?)</tr>`)

	// thumbStemRe captures the per-scene thumbnail stem (e.g. "onyxreign002"),
	// which is unique across the catalogue and used as the scene ID.
	thumbStemRe = regexp.MustCompile(`class="videothumbnail">.*?<img src="graphics/videos/([^"]+?)(?:-nn)?\.jpg"`)
	// bannerRe captures the larger banner image from the popImage() handler.
	bannerRe = regexp.MustCompile(`popImage\('graphics/videos/([^']+\.jpg)'`)

	modelLinkRe = regexp.MustCompile(`<a href="(profiles/[^"]+)"(?:\s+name="([^"]*)")?><h2 class="videomodel">([^<]*)</h2>`)
	typeRe      = regexp.MustCompile(`class="videotype">Video Type:\s*([^<]*)</span>`)
	lengthRe    = regexp.MustCompile(`class="videolength">Length:\s*([^<]*)</span>`)
	numbersRe   = regexp.MustCompile(`class="videonumbers">([^<]*)</span>`)
	dateRe      = regexp.MustCompile(`class="videodate">([^<]*)</span>`)
	descRe      = regexp.MustCompile(`(?s)class="videodescription">(.*?)</span>`)
)

type item struct {
	id          string
	profile     string
	model       string
	videoType   string
	duration    int
	setNumber   string
	date        time.Time
	description string
	thumbnail   string
}

func parseItems(body []byte) []item {
	// Trim everything before the "Upcoming" marker so commented upcoming rows
	// (and the page chrome) are never considered.
	if loc := upcomingMarkerRe.FindIndex(body); loc != nil {
		body = body[loc[1]:]
	}

	blocks := blockRe.FindAllSubmatch(body, -1)
	items := make([]item, 0, len(blocks))
	for _, b := range blocks {
		if it, ok := parseBlock(b[1]); ok {
			items = append(items, it)
		}
	}
	return items
}

func parseBlock(block []byte) (item, bool) {
	// A real video block always has both a videolength and a thumbnail stem.
	stem := thumbStemRe.FindSubmatch(block)
	if stem == nil {
		return item{}, false
	}
	if !lengthRe.Match(block) {
		return item{}, false
	}

	it := item{id: strings.TrimSpace(string(stem[1]))}

	if m := bannerRe.FindSubmatch(block); m != nil {
		it.thumbnail = defaultBase + "/graphics/videos/" + string(m[1])
	}

	if m := modelLinkRe.FindSubmatch(block); m != nil {
		it.profile = string(m[1])
		it.model = cleanText(string(m[3]))
		// "Shoot: Onyx Reign" -> "Onyx Reign"
		it.model = strings.TrimSpace(strings.TrimPrefix(it.model, "Shoot:"))
	}

	if m := typeRe.FindSubmatch(block); m != nil {
		it.videoType = cleanText(string(m[1]))
	}
	if m := lengthRe.FindSubmatch(block); m != nil {
		it.duration = parseutil.ParseDurationColon(strings.TrimSpace(string(m[1])))
	}
	if m := numbersRe.FindSubmatch(block); m != nil {
		it.setNumber = cleanText(string(m[1]))
	}
	if m := dateRe.FindSubmatch(block); m != nil {
		if d, err := parseutil.TryParseDate(cleanText(string(m[1])), dateLayout); err == nil {
			it.date = d
		}
	}
	if m := descRe.FindSubmatch(block); m != nil {
		it.description = cleanText(string(m[1]))
	}

	return it, true
}

func cleanText(s string) string {
	return strings.TrimSpace(html.UnescapeString(strings.Join(strings.Fields(s), " ")))
}

func (s *Scraper) toScene(it item, studioURL string, now time.Time) models.Scene {
	title := it.model
	if it.setNumber != "" {
		title = fmt.Sprintf("%s - %s", it.model, it.setNumber)
	}
	if title == "" {
		title = it.id
	}

	// Stable scene URL: anchor into the catalogue page by the per-scene id,
	// which matches the block's name="<id>" attribute when present.
	sceneURL := s.base + videosPath + "#" + it.id

	scene := models.Scene{
		ID:          it.id,
		SiteID:      siteID,
		StudioURL:   studioURL,
		Title:       title,
		URL:         sceneURL,
		Date:        it.date,
		Description: it.description,
		Thumbnail:   it.thumbnail,
		Duration:    it.duration,
		Studio:      studioName,
		ScrapedAt:   now,
	}
	if it.model != "" {
		scene.Performers = []string{it.model}
	}
	if it.videoType != "" {
		scene.Tags = []string{it.videoType}
	}
	return scene
}
