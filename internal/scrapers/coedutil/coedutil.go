// Package coedutil scrapes the Coed Productions network (Nebraska Coeds, Miss
// Pussycat, After Hours Exposed) — NATS "updateItem" tour sites.
//
// Listing: {SiteBase}/categories/updates_{N}_d.html. Each updateItem card
// carries the full per-scene metadata (title, performers, availability date
// and thumbnail), so no detail-page fetch is needed. The numeric/dated content
// id embedded in the thumbnail path (content/{id}/1.jpg) is the stable scene
// id. Pages past the last repeat boilerplate without content thumbnails, so a
// card with no thumbnail is skipped and an all-empty page stops the loop.
package coedutil

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
	"github.com/Wasylq/FSS/scraper"
)

type SiteConfig struct {
	ID       string
	Studio   string
	SiteBase string // e.g. "https://tour.nebraskacoeds.com" — no trailing slash
	Patterns []string
	MatchRe  *regexp.Regexp
}

type Scraper struct {
	cfg    SiteConfig
	Client *http.Client
}

func New(cfg SiteConfig) *Scraper {
	return &Scraper{cfg: cfg, Client: httpx.NewClient(30 * time.Second)}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func (s *Scraper) ID() string               { return s.cfg.ID }
func (s *Scraper) Patterns() []string       { return s.cfg.Patterns }
func (s *Scraper) MatchesURL(u string) bool { return s.cfg.MatchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	cardSplitRe = regexp.MustCompile(`class="updateItem"`)
	thumbRe     = regexp.MustCompile(`src0_1x="([^"]+)"`)
	contentIDRe = regexp.MustCompile(`content/([^/"?]+)`)
	// titleRe matches the heading link text (h5 on the trailer themes, h4 on
	// the gallery theme).
	titleRe   = regexp.MustCompile(`(?s)<h[45]>\s*<a[^>]*>(.*?)</a>`)
	hrefRe    = regexp.MustCompile(`<a\s+href="([^"]+)"`)
	trailerRe = regexp.MustCompile(`<a\s+href="([^"]*(?:/trailers/|/updates/)[^"]*\.html)"`)
	modelsRe  = regexp.MustCompile(`(?s)tour_update_models"?>(.*?)</span>`)
	modelRe   = regexp.MustCompile(`<a[^>]*>([^<]+)</a>`)
	availRe   = regexp.MustCompile(`class="availdate"[^>]*>\s*(\d{2}/\d{2}/\d{4})`)
	dateRe    = regexp.MustCompile(`(\d{2}/\d{2}/\d{4})`)

	tagStripRe = regexp.MustCompile(`<[^>]+>`)
)

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	seen := make(map[string]bool)
	scraper.Paginate(ctx, opts, s.cfg.ID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := fmt.Sprintf("%s/categories/updates_%d_d.html", s.cfg.SiteBase, page)
		cards, err := s.fetchCards(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}
		scenes := make([]models.Scene, 0, len(cards))
		for _, c := range cards {
			sc, ok := s.toScene(studioURL, c, now)
			if !ok || seen[sc.ID] {
				continue
			}
			seen[sc.ID] = true
			scenes = append(scenes, sc)
		}
		return scraper.PageResult{Scenes: scenes}, nil
	})
}

func (s *Scraper) fetchCards(ctx context.Context, pageURL string) ([]string, error) {
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL:     pageURL,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, err
	}
	body, err := func() ([]byte, error) {
		defer func() { _ = resp.Body.Close() }()
		return httpx.ReadBody(resp.Body)
	}()
	if err != nil {
		return nil, err
	}
	parts := cardSplitRe.Split(string(body), -1)
	if len(parts) <= 1 {
		return nil, nil
	}
	return parts[1:], nil
}

func (s *Scraper) toScene(studioURL, card string, now time.Time) (models.Scene, bool) {
	th := thumbRe.FindStringSubmatch(card)
	if th == nil {
		return models.Scene{}, false // boilerplate card without a content thumbnail
	}
	id := ""
	if m := contentIDRe.FindStringSubmatch(th[1]); m != nil {
		id = m[1]
	}
	if id == "" {
		return models.Scene{}, false
	}

	scene := models.Scene{
		ID:        id,
		SiteID:    s.cfg.ID,
		StudioURL: studioURL,
		Thumbnail: s.absURL(th[1]),
		Studio:    s.cfg.Studio,
		ScrapedAt: now,
	}
	if m := titleRe.FindStringSubmatch(card); m != nil {
		scene.Title = cleanText(m[1])
	}
	// Prefer the dedicated trailer/update page link; fall back to the first
	// anchor (the gallery fancybox link on photo-only sites).
	if m := trailerRe.FindStringSubmatch(card); m != nil {
		scene.URL = s.absURL(m[1])
	} else if m := hrefRe.FindStringSubmatch(card); m != nil {
		scene.URL = s.absURL(m[1])
	}
	if m := modelsRe.FindStringSubmatch(card); m != nil {
		scene.Performers = uniqueText(modelRe.FindAllStringSubmatch(m[1], -1))
	}
	scene.Date = parseDate(card)
	return scene, true
}

func parseDate(card string) time.Time {
	raw := ""
	if m := availRe.FindStringSubmatch(card); m != nil {
		raw = m[1]
	} else if m := dateRe.FindStringSubmatch(card); m != nil {
		raw = m[1]
	}
	if raw == "" {
		return time.Time{}
	}
	if t, err := time.Parse("01/02/2006", raw); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

func uniqueText(matches [][]string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range matches {
		v := cleanText(m[1])
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// absURL resolves a thumbnail/scene URL. Absolute URLs (including the gallery
// CDN host on Miss Pussycat) pass through; relative paths resolve against the
// site base.
func (s *Scraper) absURL(u string) string {
	if strings.HasPrefix(u, "http") {
		return u
	}
	if strings.HasPrefix(u, "//") {
		return "https:" + u
	}
	return s.cfg.SiteBase + "/" + strings.TrimPrefix(u, "/")
}

func cleanText(s string) string {
	s = tagStripRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}
