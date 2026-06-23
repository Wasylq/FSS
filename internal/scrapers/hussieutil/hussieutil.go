// Package hussieutil scrapes the Hussie Pass network (Hussie Pass, POV
// Pornstars, Interracial POVs, Hot and Tatted) — povporncash NATS tour sites
// whose listing cards carry the full per-scene metadata (title, date,
// duration, thumbnail), so no detail-page fetch is needed.
//
// Listing: {SiteBase}{TourPrefix}/categories/movies/{page}/latest/
// Scene:   {SiteBase}{TourPrefix}/trailers/{slug}.html
package hussieutil

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

type SiteConfig struct {
	ID         string
	Studio     string
	SiteBase   string // e.g. "https://hussiepass.com" — no trailing slash
	TourPrefix string // "" or "/tour"
	Patterns   []string
	MatchRe    *regexp.Regexp
}

type Scraper struct {
	cfg    SiteConfig
	Client *http.Client
}

func New(cfg SiteConfig) *Scraper {
	return &Scraper{
		cfg:    cfg,
		Client: httpx.NewClient(30 * time.Second),
	}
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
	cardSplitRe = regexp.MustCompile(`<div class="item-video`)
	trailerRe   = regexp.MustCompile(`<a\s+href="([^"]*/trailers/([A-Za-z0-9][A-Za-z0-9_-]*)\.html)"[^>]*title="([^"]*)"`)
	thumbRe     = regexp.MustCompile(`src0_1x="([^"]+)"`)
	contentIDRe = regexp.MustCompile(`/contentthumbs/\d+/\d+/(\d+)`)
	timeRe      = regexp.MustCompile(`class="time">\s*([0-9:]+)\s*<`)
	dateRe      = regexp.MustCompile(`class="date">\s*(\d{4}-\d{2}-\d{2})\s*<`)
)

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, s.cfg.ID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := fmt.Sprintf("%s%s/categories/movies/%d/latest/", s.cfg.SiteBase, s.cfg.TourPrefix, page)
		cards, err := s.fetchCards(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}
		scenes := make([]models.Scene, 0, len(cards))
		for _, c := range cards {
			if sc, ok := s.toScene(studioURL, c, now); ok {
				scenes = append(scenes, sc)
			}
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
	m := trailerRe.FindStringSubmatch(card)
	if m == nil {
		return models.Scene{}, false
	}
	url := m[1]
	if !strings.HasPrefix(url, "http") {
		url = s.cfg.SiteBase + "/" + strings.TrimPrefix(url, "/")
	}
	scene := models.Scene{
		ID:        m[2],
		SiteID:    s.cfg.ID,
		StudioURL: studioURL,
		Title:     html.UnescapeString(strings.TrimSpace(m[3])),
		URL:       url,
		Studio:    s.cfg.Studio,
		ScrapedAt: now,
	}
	if th := thumbRe.FindStringSubmatch(card); th != nil {
		thumb := th[1]
		if !strings.HasPrefix(thumb, "http") {
			// src0_1x is root-absolute (already includes any /tour prefix).
			thumb = s.cfg.SiteBase + "/" + strings.TrimPrefix(thumb, "/")
		}
		scene.Thumbnail = thumb
		if id := contentIDRe.FindStringSubmatch(th[1]); id != nil {
			scene.ID = id[1]
		}
	}
	if t := timeRe.FindStringSubmatch(card); t != nil {
		scene.Duration = parseutil.ParseDurationColon(t[1])
	}
	if d := dateRe.FindStringSubmatch(card); d != nil {
		if t, err := time.Parse("2006-01-02", d[1]); err == nil {
			scene.Date = t.UTC()
		}
	}
	return scene, true
}
