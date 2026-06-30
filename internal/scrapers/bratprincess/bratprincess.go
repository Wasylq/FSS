// Package bratprincess scrapes Brat Princess (bratprincess.us), a Drupal site.
// The deep, paginated archive lives at /video-list?page=N (20 nodes per page);
// the homepage /last_videos block ignores the page parameter and only shows the
// three latest, so it is not used. Each node carries the content slug (in the
// `about` attribute), the title, a poster and a body synopsis. The listing has
// no publish date or runtime, so those are left zero.
package bratprincess

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

var siteBase = "https://www.bratprincess.us"

const siteID = "bratprincess"

type Scraper struct{ Client *http.Client }

func New() *Scraper { return &Scraper{Client: httpx.NewClient(30 * time.Second)} }

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }
func (s *Scraper) Patterns() []string {
	return []string{"bratprincess.us", "bratprincess.us/video-list"}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?bratprincess\.us`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	nodeOpenRe = regexp.MustCompile(`<div[^>]*about="(/content/[^"]+)"[^>]*node node-videos`)
	titleRe    = regexp.MustCompile(`(?s)field-name-title.*?<h6>(.*?)</h6>`)
	posterRe   = regexp.MustCompile(`(?s)field-name-field-poster.*?<img[^>]*src="([^"]+)"`)
	bodyRe     = regexp.MustCompile(`(?s)field-name-body.*?property="content:encoded">(.*?)</div>`)
	tagStripRe = regexp.MustCompile(`<[^>]+>`)
)

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		// Drupal pager is 0-based; map FSS's 1-based page number onto it.
		pageURL := fmt.Sprintf("%s/video-list?page=%d", siteBase, page-1)
		body, err := s.get(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}
		scenes := parseListing(string(body), studioURL, now)
		return scraper.PageResult{Scenes: scenes}, nil
	})
}

func parseListing(doc, studioURL string, now time.Time) []models.Scene {
	idx := nodeOpenRe.FindAllStringSubmatchIndex(doc, -1)
	scenes := make([]models.Scene, 0, len(idx))
	for i, m := range idx {
		path := doc[m[2]:m[3]]
		start := m[1]
		end := len(doc)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		if sc, ok := toScene(path, doc[start:end], studioURL, now); ok {
			scenes = append(scenes, sc)
		}
	}
	return scenes
}

func toScene(path, block, studioURL string, now time.Time) (models.Scene, bool) {
	id := strings.TrimPrefix(path, "/content/")
	sc := models.Scene{
		ID:        id,
		SiteID:    siteID,
		StudioURL: studioURL,
		URL:       siteBase + path,
		Studio:    "Brat Princess",
		ScrapedAt: now,
	}
	if m := titleRe.FindStringSubmatch(block); m != nil {
		sc.Title = cleanText(m[1])
	}
	if sc.Title == "" {
		return models.Scene{}, false
	}
	if m := posterRe.FindStringSubmatch(block); m != nil {
		sc.Thumbnail = absURL(m[1])
	}
	if m := bodyRe.FindStringSubmatch(block); m != nil {
		sc.Description = cleanText(m[1])
	}
	return sc, true
}

func absURL(p string) string {
	if strings.HasPrefix(p, "http") {
		return p
	}
	return siteBase + "/" + strings.TrimPrefix(p, "/")
}

func (s *Scraper) get(ctx context.Context, u string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{URL: u, Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox)})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}

func cleanText(s string) string {
	s = tagStripRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}
