// Package femjoy scrapes Femjoy (femjoy.com). The site runs Gamma's
// "MetaMaker"/_xlabs tour; the server-rendered /videos listing cards carry the
// full per-scene metadata (title, model, photographer, date, duration,
// thumbnail), so no detail-page fetch is needed. The anonymous card links
// point at /join, but the canonical scene page is /gallery/{post-id}.
package femjoy

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/parseutil"
	"github.com/Wasylq/FSS/scraper"
)

const siteBase = "https://www.femjoy.com"

type Scraper struct{ client *http.Client }

func New() *Scraper {
	// Femjoy bootstraps a session cookie via a redirect; a jar carries it
	// across hops so the bare /videos request doesn't loop.
	c := httpx.NewClient(30 * time.Second)
	if jar, err := cookiejar.New(nil); err == nil {
		c.Jar = jar
	}
	return &Scraper{client: c}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return "femjoy" }
func (s *Scraper) Patterns() []string {
	return []string{"femjoy.com", "femjoy.com/videos", "femjoy.com/gallery/{id}"}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?femjoy\.com`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	cardSplitRe = regexp.MustCompile(`<div class="post_item video"`)
	postIDRe    = regexp.MustCompile(`data-post-id="(\d+)"`)
	posterRe    = regexp.MustCompile(`data-media-poster="([^"]+)"`)
	dateRe      = regexp.MustCompile(`class="posted_on">([^<]+)</span>`)
	durationRe  = regexp.MustCompile(`fa-video"></i>\s*([0-9:]+)`)
	titleRe     = regexp.MustCompile(`<h1><a[^>]+title="([^"]+)"`)
	h2Re        = regexp.MustCompile(`(?s)<h2>(.*?)</h2>`)
	h2LinkRe    = regexp.MustCompile(`<a[^>]+title="([^"]+)"`)
)

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, "femjoy", out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		cards, err := s.fetchCards(ctx, fmt.Sprintf("%s/videos?page=%d", siteBase, page))
		if err != nil {
			return scraper.PageResult{}, err
		}
		scenes := make([]models.Scene, 0, len(cards))
		for _, c := range cards {
			if sc, ok := toScene(studioURL, c, now); ok {
				scenes = append(scenes, sc)
			}
		}
		return scraper.PageResult{Scenes: scenes}, nil
	})
}

func (s *Scraper) fetchCards(ctx context.Context, pageURL string) ([]string, error) {
	headers := httpx.BrowserHeaders(httpx.UserAgentFirefox)
	headers["Cookie"] = "locale=en; country=US"
	resp, err := httpx.Do(ctx, s.client, httpx.Request{URL: pageURL, Headers: headers})
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

func toScene(studioURL, card string, now time.Time) (models.Scene, bool) {
	m := postIDRe.FindStringSubmatch(card)
	if m == nil {
		return models.Scene{}, false
	}
	id := m[1]
	scene := models.Scene{
		ID:        id,
		SiteID:    "femjoy",
		StudioURL: studioURL,
		URL:       siteBase + "/gallery/" + id,
		Studio:    "Femjoy",
		ScrapedAt: now,
	}
	if t := titleRe.FindStringSubmatch(card); t != nil {
		scene.Title = html.UnescapeString(strings.TrimSpace(t[1]))
	}
	if p := posterRe.FindStringSubmatch(card); p != nil {
		scene.Thumbnail = p[1]
	}
	if d := dateRe.FindStringSubmatch(card); d != nil {
		if t, err := time.Parse("Jan 2, 2006", strings.TrimSpace(d[1])); err == nil {
			scene.Date = t.UTC()
		}
	}
	if dur := durationRe.FindStringSubmatch(card); dur != nil {
		scene.Duration = parseutil.ParseDurationColon(dur[1])
	}
	if h2 := h2Re.FindStringSubmatch(card); h2 != nil {
		links := h2LinkRe.FindAllStringSubmatch(h2[1], -1)
		if len(links) > 0 {
			scene.Performers = []string{html.UnescapeString(strings.TrimSpace(links[0][1]))}
		}
		if len(links) > 1 {
			scene.Director = html.UnescapeString(strings.TrimSpace(links[len(links)-1][1]))
		}
	}
	return scene, true
}
