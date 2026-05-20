package metartutil

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const (
	cdnBase  = "https://gccdn.metartnetwork.com"
	pageSize = 30
)

type SiteConfig struct {
	SiteID     string
	Domain     string
	StudioName string
}

type Scraper struct {
	client *http.Client
	base   string
	Config SiteConfig
}

func NewScraper(cfg SiteConfig) *Scraper {
	return &Scraper{
		client: httpx.NewClient(30 * time.Second),
		base:   "https://www." + cfg.Domain,
		Config: cfg,
	}
}

func (s *Scraper) ID() string { return s.Config.SiteID }

func (s *Scraper) Patterns() []string {
	return []string{s.Config.Domain}
}

func (s *Scraper) MatchesURL(u string) bool {
	d := s.Config.Domain
	return strings.Contains(u, "://"+d) || strings.Contains(u, "://www."+d)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

type apiResponse struct {
	Total     int       `json:"total"`
	Galleries []gallery `json:"galleries"`
}

type gallery struct {
	UUID           string      `json:"UUID"`
	Name           string      `json:"name"`
	PublishedAt    string      `json:"publishedAt"`
	Type           string      `json:"type"`
	Models         []apiModel  `json:"models"`
	Photographers  []apiPerson `json:"photographers"`
	Categories     []string    `json:"categories"`
	Runtime        int         `json:"runtime"`
	Path           string      `json:"path"`
	CoverImagePath string      `json:"coverImagePath"`
}

type apiModel struct {
	UUID string `json:"UUID"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type apiPerson struct {
	UUID string `json:"UUID"`
	Name string `json:"name"`
}

func (s *Scraper) run(ctx context.Context, _ string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	progressSent := false
	for page := 1; ; page++ {
		if ctx.Err() != nil {
			return
		}
		if page > 1 && opts.Delay > 0 {
			select {
			case <-time.After(opts.Delay):
			case <-ctx.Done():
				return
			}
		}

		resp, err := s.fetchPage(ctx, page)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		if !progressSent && resp.Total > 0 {
			progressSent = true
			select {
			case out <- scraper.Progress(resp.Total):
			case <-ctx.Done():
				return
			}
		}

		if len(resp.Galleries) == 0 {
			return
		}

		now := time.Now().UTC()
		for _, g := range resp.Galleries {
			if g.Type != "MOVIE" {
				continue
			}
			if opts.KnownIDs[g.UUID] {
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
				return
			}
			select {
			case out <- scraper.Scene(toScene(s.Config, s.base, g, now)):
			case <-ctx.Done():
				return
			}
		}

		if len(resp.Galleries) < pageSize {
			return
		}
	}
}

func (s *Scraper) fetchPage(ctx context.Context, page int) (*apiResponse, error) {
	u := fmt.Sprintf("%s/api/updates?page=%d&limit=%d", s.base, page, pageSize)
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: u,
		Headers: func() map[string]string {
			h := httpx.BrowserHeaders(httpx.UserAgentFirefox)
			h["Accept"] = "application/json"
			return h
		}(),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result apiResponse
	if err := httpx.DecodeJSON(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &result, nil
}

func toScene(cfg SiteConfig, base string, g gallery, now time.Time) models.Scene {
	sc := models.Scene{
		ID:         g.UUID,
		SiteID:     cfg.SiteID,
		StudioURL:  base,
		Title:      g.Name,
		URL:        base + g.Path,
		Duration:   g.Runtime,
		Performers: modelNames(g.Models),
		Tags:       g.Categories,
		Studio:     cfg.StudioName,
		ScrapedAt:  now,
	}
	if g.CoverImagePath != "" {
		sc.Thumbnail = cdnBase + g.CoverImagePath
	}
	if len(g.Photographers) > 0 {
		sc.Director = g.Photographers[0].Name
	}
	if t, err := time.Parse(time.RFC3339Nano, g.PublishedAt); err == nil {
		sc.Date = t.UTC()
	}
	return sc
}

func modelNames(ms []apiModel) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.Name
	}
	return out
}
