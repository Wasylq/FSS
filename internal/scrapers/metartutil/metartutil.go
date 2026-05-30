package metartutil

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
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
	MoviesOnly bool // use /api/movies endpoint; accept GALLERY type (errotica-archives)
}

type Scraper struct {
	client  *http.Client
	base    string
	matchRe *regexp.Regexp
	cfg     SiteConfig
}

func New(cfg SiteConfig) *Scraper {
	return &Scraper{
		client:  httpx.NewClient(30 * time.Second),
		base:    "https://www." + cfg.Domain,
		matchRe: regexp.MustCompile(`^https?://(?:www\.)?` + regexp.QuoteMeta(cfg.Domain) + `(?:/|$)`),
		cfg:     cfg,
	}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func (s *Scraper) ID() string { return s.cfg.SiteID }

func (s *Scraper) Patterns() []string {
	return []string{s.cfg.Domain}
}

func (s *Scraper) MatchesURL(u string) bool {
	return s.matchRe.MatchString(u)
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

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, s.cfg.SiteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		resp, err := s.fetchPage(ctx, page)
		if err != nil {
			return scraper.PageResult{}, err
		}
		var scenes []models.Scene
		for _, g := range resp.Galleries {
			if !s.cfg.MoviesOnly && g.Type != "MOVIE" {
				continue
			}
			scenes = append(scenes, toScene(s.cfg, studioURL, s.base, g, now))
		}
		return scraper.PageResult{
			Scenes: scenes,
			Total:  resp.Total,
			Done:   len(resp.Galleries) < pageSize,
		}, nil
	})
}

func (s *Scraper) fetchPage(ctx context.Context, page int) (*apiResponse, error) {
	endpoint := "updates"
	if s.cfg.MoviesOnly {
		endpoint = "movies"
	}
	u := fmt.Sprintf("%s/api/%s?page=%d&limit=%d", s.base, endpoint, page, pageSize)
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

func toScene(cfg SiteConfig, studioURL, base string, g gallery, now time.Time) models.Scene {
	sc := models.Scene{
		ID:         g.UUID,
		SiteID:     cfg.SiteID,
		StudioURL:  studioURL,
		Title:      g.Name,
		URL:        base + g.Path,
		Duration:   max(g.Runtime, 0),
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
