// Package pervcityutil is the shared scraper for the PervCity network — three
// anal-focused sites (pervcity.com, analoverdose.com, upherasshole.com) running
// the same NATS/ElevatedX "videoBlock" tour template. Every scene's metadata
// (title, performers, date, runtime, description, thumbnail) is published on the
// listing card, so there is no per-scene detail fetch: the card is parsed in
// full into a models.Scene.
//
// The sites are structurally identical except for two things, both captured in
// SiteConfig: the bare domain/host, and the page-1 listing path stem
// ("updates" on pervcity.com, "movies" on the other two). The listing URL is
// "{Host}/categories/{PathStem}_{N}_d.html" with a 1-based page number; the
// markup of an individual card varies slightly between sites (heading tag,
// presence of a date/description block, runtime suffix), so the card parser is
// written to tolerate those differences rather than assume a fixed shape.
package pervcityutil

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/parseutil"
	"github.com/Wasylq/FSS/scraper"
)

// perPage is the nominal number of cards per listing page; used only to
// estimate a progress total from the highest page number in the pager.
const perPage = 12

type SiteConfig struct {
	SiteID     string
	Domain     string // bare domain, e.g. "pervcity.com"
	Host       string // full base, e.g. "https://pervcity.com"
	StudioName string
	PathStem   string // listing path stem, e.g. "updates" or "movies"
}

type Scraper struct {
	Client *http.Client
	cfg    SiteConfig
}

func New(cfg SiteConfig) *Scraper {
	return &Scraper{Client: httpx.NewClient(30 * time.Second), cfg: cfg}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func (s *Scraper) ID() string { return s.cfg.SiteID }

func (s *Scraper) Patterns() []string {
	return []string{
		s.cfg.Domain,
		s.cfg.Domain + "/categories/" + s.cfg.PathStem + "_{N}_d.html",
		s.cfg.Domain + "/models/{slug}.html",
	}
}

func (s *Scraper) MatchesURL(u string) bool {
	d := regexp.QuoteMeta(s.cfg.Domain)
	return regexp.MustCompile(`^https?://(?:www\.)?` + d + `(?:/|$)`).MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	// Model page: a single page of the same card format, no pagination.
	if strings.Contains(studioURL, "/models/") {
		scraper.Debugf(1, "%s: scraping model page %s", s.cfg.SiteID, studioURL)
		body, err := s.fetchPage(ctx, studioURL)
		if err != nil {
			select {
			case out <- scraper.Error(err):
			case <-ctx.Done():
			}
			return
		}
		scenes := s.parseListing(body, studioURL)
		select {
		case out <- scraper.Progress(len(scenes)):
		case <-ctx.Done():
			return
		}
		for _, sc := range scenes {
			if opts.KnownIDs[sc.ID] {
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
				return
			}
			select {
			case out <- scraper.Scene(sc):
			case <-ctx.Done():
				return
			}
		}
		return
	}

	scraper.Paginate(ctx, opts, s.cfg.SiteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := fmt.Sprintf("%s/categories/%s_%d_d.html", s.cfg.Host, s.cfg.PathStem, page)
		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}
		scenes := s.parseListing(body, studioURL)
		var total int
		if page == 1 {
			total = estimateTotal(body, s.cfg.PathStem)
		}
		return scraper.PageResult{
			Scenes: scenes,
			Total:  total,
			Done:   len(scenes) == 0,
		}, nil
	})
}

var (
	blockRe = regexp.MustCompile(`<div class="videoBlock">`)
	// Trailer link — captures the slug for the ID and the absolute/relative URL.
	trailerRe = regexp.MustCompile(`href="([^"]*/trailers/([^"/]+)\.html)"`)
	// Title — the trailer anchor that carries visible text (the videoPic anchor
	// wraps only an <img>, so its text strips to empty and is skipped).
	titleAnchorRe = regexp.MustCompile(`(?s)href="[^"]*/trailers/[^"]+\.html"[^>]*>(.*?)</a>`)
	modelsSpanRe  = regexp.MustCompile(`(?s)<span class="tour_update_models">(.*?)</span>`)
	modelRe       = regexp.MustCompile(`(?s)<a[^>]*>(.*?)</a>`)
	dateRe        = regexp.MustCompile(`<div class="date">\s*(\d{2}-\d{2}-\d{4})\s*</div>`)
	runtimeRe     = regexp.MustCompile(`Runtime:\s*(\d{1,2}:\d{2}(?::\d{2})?)`)
	descRe        = regexp.MustCompile(`(?s)<p>(.*?)</p>`)
	thumbRe       = regexp.MustCompile(`src0_1x="([^"]+)"`)
	tagRe         = regexp.MustCompile(`<[^>]+>`)
)

// parseListing extracts every videoBlock card from a listing or model page and
// builds a fully-populated models.Scene from each (listing-only — no detail
// fetch).
func (s *Scraper) parseListing(body []byte, studioURL string) []models.Scene {
	page := string(body)
	locs := blockRe.FindAllStringIndex(page, -1)
	scenes := make([]models.Scene, 0, len(locs))
	now := time.Now().UTC()

	for i, loc := range locs {
		start := loc[0]
		end := len(page)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		block := page[start:end]

		m := trailerRe.FindStringSubmatch(block)
		if m == nil {
			continue
		}
		url := s.absURL(m[1])
		slug := m[2]

		scene := models.Scene{
			ID:        slug,
			SiteID:    s.cfg.SiteID,
			StudioURL: studioURL,
			URL:       url,
			Studio:    s.cfg.StudioName,
			ScrapedAt: now,
		}

		scene.Title = parseTitle(block)

		if sm := modelsSpanRe.FindStringSubmatch(block); sm != nil {
			seen := map[string]bool{}
			for _, am := range modelRe.FindAllStringSubmatch(sm[1], -1) {
				name := cleanText(am[1])
				if name == "" || seen[name] {
					continue
				}
				seen[name] = true
				scene.Performers = append(scene.Performers, name)
			}
		}

		if dm := dateRe.FindStringSubmatch(block); dm != nil {
			if t, err := time.Parse("01-02-2006", dm[1]); err == nil {
				scene.Date = t.UTC()
			}
		}

		if rm := runtimeRe.FindStringSubmatch(block); rm != nil {
			scene.Duration = parseutil.ParseDurationColon(rm[1])
		}

		if pm := descRe.FindStringSubmatch(block); pm != nil {
			scene.Description = cleanText(pm[1])
		}

		if tm := thumbRe.FindStringSubmatch(block); tm != nil {
			scene.Thumbnail = s.absURL(tm[1])
		}

		if scene.Title == "" {
			continue
		}
		scenes = append(scenes, scene)
	}
	return scenes
}

// parseTitle returns the first trailer anchor whose visible text is non-empty.
func parseTitle(block string) string {
	for _, m := range titleAnchorRe.FindAllStringSubmatch(block, -1) {
		if t := cleanText(m[1]); t != "" {
			return t
		}
	}
	return ""
}

// cleanText strips tags, unescapes entities and trims whitespace.
func cleanText(s string) string {
	s = tagRe.ReplaceAllString(s, "")
	return strings.TrimSpace(html.UnescapeString(s))
}

func (s *Scraper) absURL(u string) string {
	if strings.HasPrefix(u, "/") {
		return s.cfg.Host + u
	}
	return u
}

// estimateTotal reads the highest page number from the listing pager links
// (e.g. "updates_132_d.html") to approximate the scene count for progress.
func estimateTotal(body []byte, stem string) int {
	page := string(body)
	re := regexp.MustCompile(regexp.QuoteMeta(stem) + `_(\d+)_d\.html`)
	max := 0
	for _, m := range re.FindAllStringSubmatch(page, -1) {
		if n, _ := strconv.Atoi(m[1]); n > max {
			max = n
		}
	}
	if max == 0 {
		return 0
	}
	return max * perPage
}

func (s *Scraper) fetchPage(ctx context.Context, url string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL:     url,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}
