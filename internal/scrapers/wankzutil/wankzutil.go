package wankzutil

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

type SiteConfig struct {
	SiteID     string
	SiteBase   string // e.g. "https://www.wankz.com"
	StudioName string
}

type Scraper struct {
	Client *http.Client
	cfg    SiteConfig
}

func New(cfg SiteConfig) *Scraper {
	return &Scraper{
		Client: httpx.NewClient(30 * time.Second),
		cfg:    cfg,
	}
}

type apiResponse struct {
	Success bool    `json:"success"`
	Count   int     `json:"count"`
	Result  []Video `json:"result"`
}

type Video struct {
	ID          int      `json:"id"`
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Duration    int      `json:"duration"`
	Thumb       string   `json:"thumb"`
	Channel     string   `json:"channel"`
	Actors      []string `json:"actors"`
	Tags        []string `json:"tags"`
	ActiveDate  string   `json:"active_date"`
}

const PageSize = 50

func (s *Scraper) Run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	channel := ParseChannel(studioURL)
	if channel != "" {
		scraper.Debugf(1, "%s: detected channel filter: %s", s.cfg.SiteID, channel)
	}
	now := time.Now().UTC()

	var total int
	scraper.Paginate(ctx, opts, s.cfg.SiteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		videos, t, err := s.FetchPage(ctx, page, channel)
		if err != nil {
			return scraper.PageResult{}, err
		}
		if page == 1 {
			total = t
		}

		scenes := make([]models.Scene, len(videos))
		for i, v := range videos {
			scenes[i] = ToScene(s.cfg, studioURL, v, now)
		}

		return scraper.PageResult{
			Scenes: scenes,
			Total:  total,
			Done:   page*PageSize >= total,
		}, nil
	})
}

func (s *Scraper) FetchPage(ctx context.Context, page int, channel string) ([]Video, int, error) {
	u := fmt.Sprintf("%s/api/videos/find.json?page=%d&limit=%d&order=date",
		s.cfg.SiteBase, page, PageSize)
	if channel != "" {
		u += "&channel=" + url.QueryEscape(channel)
	}

	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL: u,
		Headers: func() map[string]string {
			h := httpx.BrowserHeaders(httpx.UserAgentFirefox)
			h["Accept"] = "application/json"
			return h
		}(),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("fetching page %d: %w", page, err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result apiResponse
	if err := httpx.DecodeJSON(resp.Body, &result); err != nil {
		return nil, 0, fmt.Errorf("decoding page %d: %w", page, err)
	}
	if !result.Success {
		return nil, 0, fmt.Errorf("API error on page %d", page)
	}
	return result.Result, result.Count, nil
}

func ToScene(cfg SiteConfig, studioURL string, v Video, now time.Time) models.Scene {
	var date time.Time
	if t, err := time.Parse("2006-01-02 15:04:05", v.ActiveDate); err == nil {
		date = t.UTC()
	}

	title := v.Title
	if title == "" {
		title = titleFromURL(v.URL)
	}
	if title == "" && len(v.Actors) > 0 {
		title = strings.Join(v.Actors, ", ")
	}
	if title == "" {
		title = strconv.Itoa(v.ID)
	}

	return models.Scene{
		ID:          strconv.Itoa(v.ID),
		SiteID:      cfg.SiteID,
		StudioURL:   studioURL,
		Title:       title,
		URL:         v.URL,
		Date:        date,
		Duration:    v.Duration,
		Performers:  v.Actors,
		Tags:        v.Tags,
		Description: v.Description,
		Thumbnail:   v.Thumb,
		Studio:      v.Channel,
		ScrapedAt:   now,
	}
}

func titleFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	slug := strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), "/")
	// Strip trailing numeric ID (e.g. "some-slug-12345" → "some-slug")
	if i := strings.LastIndex(slug, "-"); i > 0 {
		tail := slug[i+1:]
		if _, err := strconv.Atoi(tail); err == nil {
			slug = slug[:i]
		}
	}
	// Pure numeric slug (just an ID, no title info)
	if _, err := strconv.Atoi(slug); err == nil {
		return ""
	}
	if slug == "" {
		return ""
	}
	title := strings.ReplaceAll(slug, "-", " ")
	return strings.ToUpper(title[:1]) + title[1:]
}

func ParseChannel(studioURL string) string {
	u, err := url.Parse(studioURL)
	if err != nil {
		return ""
	}
	path := strings.TrimSuffix(u.Path, "/")
	if rest, ok := strings.CutPrefix(path, "/channels/"); ok && rest != "" {
		return rest
	}
	return ""
}
