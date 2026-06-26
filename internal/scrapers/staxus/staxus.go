// Package staxus scrapes Staxus (staxus.com), a gay twink studio running an
// Elevated X "trial" tour. The /trial/category.php listing renders schema.org
// microdata cards carrying the full per-scene metadata (set id, title, models,
// thumbnail), so no detail-page fetch is needed — the detail page is ~7MB.
// id=50 is the "all videos" category; s=d sorts newest-first; &page=N paginates.
package staxus

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

// siteBase is a var (not const) so tests can point it at an httptest server.
var siteBase = "https://staxus.com"

type Scraper struct{ client *http.Client }

func New() *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second)}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return "staxus" }

func (s *Scraper) Patterns() []string {
	return []string{
		"staxus.com",
		"staxus.com/trial/category.php?id={id}",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?staxus\.com`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// ---- runner ----

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, "staxus", out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := fmt.Sprintf("%s/trial/category.php?id=50&lang=0&s=d&page=%d", siteBase, page)
		cards, err := s.fetchCards(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}
		scenes := make([]models.Scene, 0, len(cards))
		for _, c := range cards {
			if sc, ok := toScene(studioURL, c, now); ok {
				scenes = append(scenes, sc)
			}
		}
		// An empty page (past the last) stops Paginate via len(Scenes) == 0.
		return scraper.PageResult{Scenes: scenes}, nil
	})
}

// ---- page fetch ----

func (s *Scraper) fetchCards(ctx context.Context, pageURL string) ([]string, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{URL: pageURL, Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox)})
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

// ---- parsing ----

var (
	// Each listing card opens with this wrapper carrying the numeric set id.
	cardSplitRe = regexp.MustCompile(`<div class="update_details" data-setid="\d+"`)
	setIDRe     = regexp.MustCompile(`gallery\.php\?id=(\d+)&type=vids`)
	titleRe     = regexp.MustCompile(`class="title_bar_movie"[^>]*href="([^"]*type=vids)"[^>]*><span itemprop="name">([^<]*)</span>`)
	thumbRe     = regexp.MustCompile(`background-image:\s*url\(([^)]+)\)`)
	modelsRe    = regexp.MustCompile(`(?s)class="update_models">(.*?)</div>`)
	actorNameRe = regexp.MustCompile(`itemprop="name">([^<]+)</span>`)
	dateRe      = regexp.MustCompile(`<span>\s*(\d{1,2}\s+[A-Z][a-z]{2}\s+\d{4})\s*</span>`)
)

func toScene(studioURL, card string, now time.Time) (models.Scene, bool) {
	// title_bar_movie's href is the canonical gallery link; only type=vids
	// items are videos — type=highres are photo sets and are skipped.
	tm := titleRe.FindStringSubmatch(card)
	if tm == nil {
		return models.Scene{}, false
	}
	idm := setIDRe.FindStringSubmatch(card)
	if idm == nil {
		return models.Scene{}, false
	}
	id := idm[1]

	scene := models.Scene{
		ID:        id,
		SiteID:    "staxus",
		StudioURL: studioURL,
		Title:     html.UnescapeString(strings.TrimSpace(tm[2])),
		URL:       siteBase + "/trial/gallery.php?id=" + id + "&type=vids",
		Studio:    "Staxus",
		ScrapedAt: now,
	}

	if t := thumbRe.FindStringSubmatch(card); t != nil {
		scene.Thumbnail = absURL(strings.TrimSpace(t[1]))
	}

	if d := dateRe.FindStringSubmatch(card); d != nil {
		if parsed, err := time.Parse("2 Jan 2006", strings.TrimSpace(d[1])); err == nil {
			scene.Date = parsed.UTC()
		}
	}

	if m := modelsRe.FindStringSubmatch(card); m != nil {
		seen := map[string]bool{}
		for _, a := range actorNameRe.FindAllStringSubmatch(m[1], -1) {
			name := html.UnescapeString(strings.TrimSpace(a[1]))
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			scene.Performers = append(scene.Performers, name)
		}
	}

	return scene, true
}

func absURL(u string) string {
	if u == "" || strings.HasPrefix(u, "http") {
		return u
	}
	return siteBase + "/" + strings.TrimPrefix(u, "/")
}
