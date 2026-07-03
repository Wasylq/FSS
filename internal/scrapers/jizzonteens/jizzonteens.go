// Package jizzonteens scrapes Jizz on Teens (jizzonteens.com), a thin PHP
// studio tour on the "Deluxe Coin" shared CDN. The listing inlines every scene
// in an <article> block carrying the slug, title, poster and description, so no
// detail-page fetch is needed. Page 1 is /, later pages are /page/{N}; pages
// past the end repeat the last page, so pagination stops when a page yields no
// new slugs. The tour exposes no reliable publish date — Date is left zero.
package jizzonteens

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
	siteID     = "jizzonteens"
	studioName = "Jizz on Teens"
)

var siteBase = "http://jizzonteens.com"

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
		"jizzonteens.com",
		"jizzonteens.com/page/{n}",
		"jizzonteens.com/content/{slug}/",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?jizzonteens\.com`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	articleSplitRe = regexp.MustCompile(`<article>`)
	slugRe         = regexp.MustCompile(`/content/([a-z0-9-]+)/thumbnails`)
	titleRe        = regexp.MustCompile(`<h2>([^<]+)</h2>`)
	posterRe       = regexp.MustCompile(`poster="([^"]+)"`)
	descRe         = regexp.MustCompile(`(?s)<textarea[^>]*>(.*?)</textarea>`)
)

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	seen := make(map[string]bool)
	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := siteBase + "/"
		if page > 1 {
			pageURL = fmt.Sprintf("%s/page/%d", siteBase, page)
		}
		items, err := s.fetchScenes(ctx, studioURL, pageURL, now)
		if err != nil {
			return scraper.PageResult{}, err
		}
		// Pages past the end repeat the last page — stop when nothing is new.
		fresh := items[:0]
		for _, sc := range items {
			if !seen[sc.ID] {
				seen[sc.ID] = true
				fresh = append(fresh, sc)
			}
		}
		if len(fresh) == 0 {
			return scraper.PageResult{Done: true}, nil
		}
		return scraper.PageResult{Scenes: fresh}, nil
	})
}

func (s *Scraper) fetchScenes(ctx context.Context, studioURL, pageURL string, now time.Time) ([]models.Scene, error) {
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
	parts := articleSplitRe.Split(string(body), -1)
	scenes := make([]models.Scene, 0, len(parts))
	for _, p := range parts[1:] {
		if sc, ok := toScene(studioURL, p, now); ok {
			scenes = append(scenes, sc)
		}
	}
	scraper.Debugf(1, "jizzonteens: listing %s -> %d scenes", pageURL, len(scenes))
	return scenes, nil
}

func toScene(studioURL, article string, now time.Time) (models.Scene, bool) {
	sm := slugRe.FindStringSubmatch(article)
	if sm == nil {
		return models.Scene{}, false
	}
	slug := sm[1]
	scene := models.Scene{
		ID:        slug,
		SiteID:    siteID,
		StudioURL: studioURL,
		URL:       siteBase + "/content/" + slug + "/",
		Studio:    studioName,
		ScrapedAt: now,
	}
	if t := titleRe.FindStringSubmatch(article); t != nil {
		scene.Title = html.UnescapeString(strings.TrimSpace(t[1]))
	}
	if p := posterRe.FindStringSubmatch(article); p != nil {
		scene.Thumbnail = absURL(p[1])
	}
	if d := descRe.FindStringSubmatch(article); d != nil {
		scene.Description = html.UnescapeString(strings.TrimSpace(d[1]))
	}
	return scene, true
}

func absURL(u string) string {
	if strings.HasPrefix(u, "http") {
		return u
	}
	return siteBase + "/" + strings.TrimPrefix(u, "/")
}
