// Package wearehairy scrapes We Are Hairy (wearehairy.com), an MMCore-CMS photo
// site. The /categories/Photos?page={N} listing renders one card per gallery
// inside a `_results_posts_item` block carrying the post id (data-post-id), the
// model link/name (title attribute) and the real cover image (the `_image` img,
// not the no_video_cover.png placeholder). Each gallery's canonical page is
// /post/details/{postid}. Listing pages are walked until one returns no cards.
package wearehairy

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
	"github.com/Wasylq/FSS/scraper"
)

// siteBase is a var (not const) so tests can point it at a local httptest server.
var siteBase = "https://www.wearehairy.com"

type Scraper struct {
	Client *http.Client
}

func New() *Scraper {
	// MMCore redirects bare requests to set a device-detection cookie; a jar
	// carries it across the www redirect so the listing request doesn't loop.
	c := httpx.NewClient(30 * time.Second)
	if jar, err := cookiejar.New(nil); err == nil {
		c.Jar = jar
	}
	return &Scraper{Client: c}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return "wearehairy" }

func (s *Scraper) Patterns() []string {
	return []string{"wearehairy.com", "wearehairy.com/categories/Photos"}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?wearehairy\.com`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	cardSplitRe = regexp.MustCompile(`_results_posts_item`)
	postIDRe    = regexp.MustCompile(`data-post-id="(\d+)"`)
	modelLinkRe = regexp.MustCompile(`href="/models/[^"]+"\s+title="([^"]+)"`)
	thumbRe     = regexp.MustCompile(`class="_image[^>]+src="([^"]+)"`)
)

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, "wearehairy", out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		cards, err := s.fetchListing(ctx, page)
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

func (s *Scraper) fetchListing(ctx context.Context, page int) ([]string, error) {
	pageURL := fmt.Sprintf("%s/categories/Photos?page=%d", siteBase, page)
	headers := httpx.BrowserHeaders(httpx.UserAgentFirefox)
	headers["Cookie"] = "device_view=full"
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{URL: pageURL, Headers: headers})
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
		SiteID:    "wearehairy",
		StudioURL: studioURL,
		URL:       siteBase + "/post/details/" + id,
		Studio:    "We Are Hairy",
		ScrapedAt: now,
	}
	if t := modelLinkRe.FindStringSubmatch(card); t != nil {
		name := html.UnescapeString(strings.TrimSpace(t[1]))
		scene.Title = name
		if name != "" {
			scene.Performers = []string{name}
		}
	}
	if scene.Title == "" {
		scene.Title = "We Are Hairy " + id
	}
	if th := thumbRe.FindStringSubmatch(card); th != nil {
		scene.Thumbnail = html.UnescapeString(strings.TrimSpace(th[1]))
	}
	return scene, true
}
