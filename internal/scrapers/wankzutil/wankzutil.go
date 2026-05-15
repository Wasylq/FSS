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
	Config SiteConfig
}

func NewScraper(cfg SiteConfig) *Scraper {
	return &Scraper{
		Client: httpx.NewClient(30 * time.Second),
		Config: cfg,
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
	now := time.Now().UTC()

	for page := 1; ; page++ {
		if ctx.Err() != nil {
			return
		}

		videos, total, err := s.FetchPage(ctx, page, channel)
		if err != nil {
			select {
			case out <- scraper.Error(err):
			case <-ctx.Done():
			}
			return
		}

		if page == 1 {
			select {
			case out <- scraper.Progress(total):
			case <-ctx.Done():
				return
			}
		}

		if len(videos) == 0 {
			return
		}

		for _, v := range videos {
			scene := ToScene(s.Config, studioURL, v, now)
			if opts.KnownIDs != nil && opts.KnownIDs[scene.ID] {
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
				return
			}
			select {
			case out <- scraper.Scene(scene):
			case <-ctx.Done():
				return
			}
		}

		if page*PageSize >= total {
			return
		}

		select {
		case <-time.After(opts.Delay):
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scraper) FetchPage(ctx context.Context, page int, channel string) ([]Video, int, error) {
	u := fmt.Sprintf("%s/api/videos/find.json?page=%d&limit=%d&order=date",
		s.Config.SiteBase, page, PageSize)
	if channel != "" {
		u += "&channel=" + url.QueryEscape(channel)
	}

	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL: u,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentFirefox,
			"Accept":     "application/json",
		},
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

	return models.Scene{
		ID:          strconv.Itoa(v.ID),
		SiteID:      cfg.SiteID,
		StudioURL:   studioURL,
		Title:       v.Title,
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
