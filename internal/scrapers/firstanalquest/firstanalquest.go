// Package firstanalquest scrapes First Anal Quest (firstanalquest.com), a
// PHP studio tour on the "Deluxe Coin" shared CDN. The /latest-updates/
// listing cards carry the full per-scene metadata (title, models, date,
// duration, quality, rotating screenshot thumbnail), so no detail-page fetch
// is needed. Page 1 is /latest-updates/, later pages are /latest-updates/{N}/;
// pagination stops when a page yields no cards.
package firstanalquest

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

const (
	siteID     = "firstanalquest"
	studioName = "First Anal Quest"
)

var siteBase = "http://www.firstanalquest.com"

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
		"firstanalquest.com",
		"firstanalquest.com/latest-updates/",
		"firstanalquest.com/videos/{slug}-{id}/",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?firstanalquest\.com`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	cardSplitRe = regexp.MustCompile(`<li class="thumb">`)
	urlRe       = regexp.MustCompile(`href="(https?://[^"]*?/videos/[^"]+)"\s+class="thumb-img"`)
	idRe        = regexp.MustCompile(`-(\d+)/?$`)
	titleRe     = regexp.MustCompile(`thumb-title">([^<]+)</span>`)
	durationRe  = regexp.MustCompile(`thumb-duration">\s*([0-9:]+)\s*</span>`)
	qualityRe   = regexp.MustCompile(`thumb-quality">\s*([^<]+?)\s*</span>`)
	imgRe       = regexp.MustCompile(`<img\s+src="([^"]+)"`)
	addedRe     = regexp.MustCompile(`thumb-added">\s*([^<]+?)\s*</span>`)
	modelsRe    = regexp.MustCompile(`(?s)thumb-models">(.*?)</span>`)
	modelLinkRe = regexp.MustCompile(`/models/[^"]+/">([^<]+)</a>`)
)

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := siteBase + "/latest-updates/"
		if page > 1 {
			pageURL = fmt.Sprintf("%s/latest-updates/%d/", siteBase, page)
		}
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
		return scraper.PageResult{Scenes: scenes}, nil
	})
}

func (s *Scraper) fetchCards(ctx context.Context, pageURL string) ([]string, error) {
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{URL: pageURL, Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox)})
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
	scraper.Debugf(1, "firstanalquest: listing %s -> %d cards", pageURL, len(parts)-1)
	if len(parts) <= 1 {
		return nil, nil
	}
	return parts[1:], nil
}

func toScene(studioURL, card string, now time.Time) (models.Scene, bool) {
	um := urlRe.FindStringSubmatch(card)
	if um == nil {
		return models.Scene{}, false
	}
	u := um[1]
	idm := idRe.FindStringSubmatch(strings.TrimSuffix(u, "/"))
	if idm == nil {
		return models.Scene{}, false
	}
	scene := models.Scene{
		ID:        idm[1],
		SiteID:    siteID,
		StudioURL: studioURL,
		URL:       u,
		Studio:    studioName,
		ScrapedAt: now,
	}
	if t := titleRe.FindStringSubmatch(card); t != nil {
		scene.Title = html.UnescapeString(strings.TrimSpace(t[1]))
	}
	if d := durationRe.FindStringSubmatch(card); d != nil {
		scene.Duration = parseutil.ParseDurationColon(d[1])
	}
	if q := qualityRe.FindStringSubmatch(card); q != nil {
		scene.Resolution = strings.TrimSpace(q[1])
	}
	if img := imgRe.FindStringSubmatch(card); img != nil {
		scene.Thumbnail = img[1]
	}
	if a := addedRe.FindStringSubmatch(card); a != nil {
		if t, err := parseutil.TryParseDate(strings.TrimSpace(a[1]), "Jan 2, 2006"); err == nil {
			scene.Date = t
		}
	}
	if mm := modelsRe.FindStringSubmatch(card); mm != nil {
		for _, link := range modelLinkRe.FindAllStringSubmatch(mm[1], -1) {
			name := html.UnescapeString(strings.TrimSpace(link[1]))
			if name != "" {
				scene.Performers = append(scene.Performers, name)
			}
		}
	}
	return scene, true
}
