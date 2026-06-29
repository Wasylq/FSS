// Package swearlutil is the shared scraper for the Swearl / VR Bangers network
// of VR sites (vrbangers.com, vrbtrans.com, blowvr.com, arporn.com, vrbgay.com).
//
// All five sites share one anonymous JSON content API hosted on a per-site
// "content." subdomain:
//
//	https://content.{domain}/api/content/v1/videos?page={N}&limit=24&sort=latest
//
// The listing response carries the full per-scene metadata (title, slug,
// publishedAt, duration, models, poster, viewCount, description) under
// data.items[], plus data.pages (total page count). Categories are only on the
// per-scene detail endpoint (data.item.categories[]), which is fetched
// best-effort; a detail-fetch failure never drops the scene.
package swearlutil

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

// SiteConfig holds the per-site values that vary across the network.
type SiteConfig struct {
	ID          string         // stable scraper ID, e.g. "vrbangers"
	ContentHost string         // API host, e.g. "content.vrbangers.com"
	SiteBase    string         // public site root, e.g. "https://vrbangers.com"
	Studio      string         // human studio name, e.g. "VR Bangers"
	MatchRe     *regexp.Regexp // URL matcher
}

// Scraper is one StudioScraper instance per network site.
type Scraper struct {
	cfg SiteConfig
	// APIBase is the base URL for the content API. It defaults to
	// "https://" + cfg.ContentHost; tests override it to point at an
	// httptest server.
	APIBase string
	// Client is the HTTP client; exported so tests can inject one.
	Client *http.Client
}

// New constructs a scraper for the given site config.
func New(cfg SiteConfig) *Scraper {
	return &Scraper{
		cfg:     cfg,
		APIBase: "https://" + cfg.ContentHost,
		Client:  httpx.NewClient(30 * time.Second),
	}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func (s *Scraper) ID() string { return s.cfg.ID }

func (s *Scraper) Patterns() []string {
	domain := strings.TrimPrefix(s.cfg.SiteBase, "https://")
	return []string{domain, domain + "/video/{slug}"}
}

func (s *Scraper) MatchesURL(u string) bool { return s.cfg.MatchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

const pageSize = 24

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, s.cfg.ID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		items, pages, err := s.fetchPage(ctx, page)
		if err != nil {
			return scraper.PageResult{}, err
		}
		scenes := make([]models.Scene, 0, len(items))
		for _, it := range items {
			sc := s.toScene(studioURL, it, now)
			// Best-effort category enrichment; never drop the scene.
			if cats := s.fetchCategories(ctx, it.Slug); len(cats) > 0 {
				sc.Categories = cats
			}
			scenes = append(scenes, sc)
		}
		return scraper.PageResult{Scenes: scenes, Total: pages * pageSize, Done: page >= pages}, nil
	})
}

// ---- API types ----

type listResponse struct {
	Data struct {
		Items []videoItem `json:"items"`
		Pages int         `json:"pages"`
	} `json:"data"`
}

type detailResponse struct {
	Data struct {
		Item struct {
			Categories []struct {
				Name string `json:"name"`
			} `json:"categories"`
		} `json:"item"`
	} `json:"data"`
}

type videoItem struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Slug          string `json:"slug"`
	Description   string `json:"description"`
	PublishedAt   int64  `json:"publishedAt"`
	ViewCount     int    `json:"viewCount"`
	VideoSettings struct {
		Duration int `json:"duration"`
	} `json:"videoSettings"`
	Models []struct {
		Slug  string `json:"slug"`
		Title string `json:"title"`
	} `json:"models"`
	Poster struct {
		Permalink string `json:"permalink"`
	} `json:"poster"`
}

func (s *Scraper) fetchPage(ctx context.Context, page int) ([]videoItem, int, error) {
	u := fmt.Sprintf("%s/api/content/v1/videos?page=%d&limit=%d&sort=latest", s.APIBase, page, pageSize)
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL:     u,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, 0, err
	}
	var lr listResponse
	if err := func() error {
		defer func() { _ = resp.Body.Close() }()
		return httpx.DecodeJSON(resp.Body, &lr)
	}(); err != nil {
		return nil, 0, fmt.Errorf("decode list: %w", err)
	}
	return lr.Data.Items, lr.Data.Pages, nil
}

// fetchCategories does a best-effort per-scene detail fetch for categories.
// Any error returns nil so the caller keeps the scene without categories.
func (s *Scraper) fetchCategories(ctx context.Context, slug string) []string {
	if slug == "" {
		return nil
	}
	u := fmt.Sprintf("%s/api/content/v1/videos/%s", s.APIBase, slug)
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL:     u,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		scraper.Debugf(1, "%s: detail fetch %s failed: %v", s.cfg.ID, slug, err)
		return nil
	}
	var dr detailResponse
	if err := func() error {
		defer func() { _ = resp.Body.Close() }()
		return httpx.DecodeJSON(resp.Body, &dr)
	}(); err != nil {
		scraper.Debugf(1, "%s: detail decode %s failed: %v", s.cfg.ID, slug, err)
		return nil
	}
	var cats []string
	for _, c := range dr.Data.Item.Categories {
		if n := strings.TrimSpace(c.Name); n != "" {
			cats = append(cats, n)
		}
	}
	return cats
}

var tagRe = regexp.MustCompile(`<[^>]+>`)

// cleanText strips HTML tags and unescapes entities.
func cleanText(s string) string {
	s = tagRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return strings.TrimSpace(s)
}

func (s *Scraper) toScene(studioURL string, it videoItem, now time.Time) models.Scene {
	scene := models.Scene{
		ID:          it.ID,
		SiteID:      s.cfg.ID,
		StudioURL:   studioURL,
		Title:       html.UnescapeString(strings.TrimSpace(it.Title)),
		URL:         fmt.Sprintf("%s/video/%s/", s.cfg.SiteBase, it.Slug),
		Description: cleanText(it.Description),
		Studio:      s.cfg.Studio,
		Duration:    it.VideoSettings.Duration,
		Views:       it.ViewCount,
		ScrapedAt:   now,
	}
	if it.ID == "" {
		scene.ID = it.Slug
	}
	if it.PublishedAt > 0 {
		scene.Date = time.Unix(it.PublishedAt, 0).UTC()
	}
	if p := strings.TrimSpace(it.Poster.Permalink); p != "" {
		scene.Thumbnail = s.APIBase + p
	}
	for _, m := range it.Models {
		if n := strings.TrimSpace(m.Title); n != "" {
			scene.Performers = append(scene.Performers, n)
		}
	}
	return scene
}
