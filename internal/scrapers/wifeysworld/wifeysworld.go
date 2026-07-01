// Package wifeysworld scrapes Wifey's World (wifeysworld.com), a NATS tour
// site. The splash page redirects to /v3/tour/; the update listing lives at
// /v3/tour/categories/updates_{N}_d.html. Each card carries the full per-scene
// metadata (set id, title, publish date, thumbnail) and links only to the join
// page — there is no public per-scene detail page, so this scraper is
// listing-only.
package wifeysworld

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

const (
	siteID = "wifeysworld"
	studio = "Wifey's World"
)

// baseURL is a var so tests can point it at an httptest server.
var baseURL = "https://wifeysworld.com"

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?wifeysworld\.com`)

type Scraper struct {
	Client *http.Client
}

func New() *Scraper {
	return &Scraper{Client: httpx.NewClient(30 * time.Second)}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }
func (s *Scraper) Patterns() []string {
	return []string{
		"wifeysworld.com",
		"wifeysworld.com/v3/tour/categories/updates_{N}_d.html",
	}
}
func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	cardSplitRe = regexp.MustCompile(`<div class="update_details`)
	setIDRe     = regexp.MustCompile(`data-setid="(\d+)"`)
	titleRe     = regexp.MustCompile(`card-custom__title">([^<]*)<`)
	dateRe      = regexp.MustCompile(`card-section-date">\s*(\d{2}/\d{2}/\d{4})`)
	thumbRe     = regexp.MustCompile(`src0_1x="([^"]+)"`)
)

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := fmt.Sprintf("%s/v3/tour/categories/updates_%d_d.html", baseURL, page)
		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}
		scenes := parseListing(body, studioURL, now)
		return scraper.PageResult{Scenes: scenes}, nil
	})
}

func parseListing(body []byte, studioURL string, now time.Time) []models.Scene {
	parts := cardSplitRe.Split(string(body), -1)
	if len(parts) <= 1 {
		return nil
	}
	scenes := make([]models.Scene, 0, len(parts)-1)
	for _, card := range parts[1:] {
		m := setIDRe.FindStringSubmatch(card)
		if m == nil {
			continue
		}
		scene := models.Scene{
			ID:        m[1],
			SiteID:    siteID,
			StudioURL: studioURL,
			URL:       baseURL + "/v3/tour/",
			Studio:    studio,
			ScrapedAt: now,
		}
		if t := titleRe.FindStringSubmatch(card); t != nil {
			scene.Title = html.UnescapeString(strings.TrimSpace(t[1]))
		}
		if d := dateRe.FindStringSubmatch(card); d != nil {
			if ts, err := time.Parse("01/02/2006", d[1]); err == nil {
				scene.Date = ts.UTC()
			}
		}
		if th := thumbRe.FindStringSubmatch(card); th != nil {
			thumb := th[1]
			if !strings.HasPrefix(thumb, "http") {
				thumb = baseURL + "/" + strings.TrimPrefix(thumb, "/")
			}
			scene.Thumbnail = thumb
		}
		scenes = append(scenes, scene)
	}
	return scenes
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
