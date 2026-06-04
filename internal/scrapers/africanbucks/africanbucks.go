package africanbucks

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const (
	siteID  = "africanbucks"
	perPage = 1000
)

var matchRe = regexp.MustCompile(
	`^https?://(?:www\.)?(africanbucks|africancasting|africanfucktour|africanlesbians|` +
		`africansextrip|africangf|analfucktour|blackfucktour|facefucktour|fuckmyjeans|` +
		`latinacasting|latinafucktour|realafricans|ripherup|sexpacker)\.com`)

var thumbDateRe = regexp.MustCompile(`(\d{4}-\d{2}-\d{2})-`)

type Scraper struct {
	client *http.Client
	base   string
}

func New() *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second)}
}

func (s *Scraper) apiBase(studioURL string) (string, error) {
	if s.base != "" {
		return s.base, nil
	}
	return apiBase(studioURL)
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		"africanbucks.com/",
		"africancasting.com/",
		"africanfucktour.com/",
		"africanlesbians.com/",
		"africansextrip.com/",
		"africangf.com/",
		"analfucktour.com/",
		"blackfucktour.com/",
		"facefucktour.com/",
		"fuckmyjeans.com/",
		"latinacasting.com/",
		"latinafucktour.com/",
		"realafricans.com/",
		"ripherup.com/",
		"sexpacker.com/",
	}
}

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

func apiBase(studioURL string) (string, error) {
	u, err := url.Parse(studioURL)
	if err != nil {
		return "", err
	}
	host := strings.TrimPrefix(u.Hostname(), "www.")
	if host == "africanbucks.com" {
		host = "africancasting.com"
	}
	return "https://members." + host, nil
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	base, err := s.apiBase(studioURL)
	if err != nil {
		select {
		case out <- scraper.Error(fmt.Errorf("invalid studio URL: %w", err)):
		case <-ctx.Done():
		}
		return
	}

	scraper.Debugf(1, "%s: using API at %s", siteID, base)

	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		offset := (page - 1) * perPage

		videos, total, err := s.fetchVideos(ctx, base, offset)
		if err != nil {
			return scraper.PageResult{}, err
		}

		scenes := make([]models.Scene, len(videos))
		for i, v := range videos {
			scenes[i] = toScene(v, studioURL, now)
		}

		return scraper.PageResult{
			Scenes: scenes,
			Total:  total,
			Done:   offset+len(videos) >= total,
		}, nil
	})
}

type apiResponse struct {
	Success      bool       `json:"success"`
	TotalResults int        `json:"total_results"`
	Data         []apiVideo `json:"data"`
}

type apiVideo struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Length      string `json:"length"`
	Description string `json:"description"`
	Channels    string `json:"channels"`
	Models      string `json:"models"`
	URL         string `json:"url"`
	MainThumb   string `json:"main_thumb"`
}

func (s *Scraper) fetchVideos(ctx context.Context, base string, offset int) ([]apiVideo, int, error) {
	apiURL := fmt.Sprintf("%s/api/?output=json&command=media.newest&type=videos&offset=%d&amount=%d",
		base, offset, perPage)

	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     apiURL,
		Headers: httpx.BrowserHeaders(httpx.UserAgentChrome),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("fetch offset %d: %w", offset, err)
	}
	defer func() { _ = resp.Body.Close() }()

	var r apiResponse
	if err := httpx.DecodeJSON(resp.Body, &r); err != nil {
		return nil, 0, fmt.Errorf("decode response: %w", err)
	}

	if !r.Success {
		return nil, 0, fmt.Errorf("API returned success=false at offset %d", offset)
	}

	return r.Data, r.TotalResults, nil
}

func toScene(v apiVideo, studioURL string, now time.Time) models.Scene {
	sc := models.Scene{
		ID:        v.ID,
		SiteID:    siteID,
		StudioURL: studioURL,
		Title:     strings.TrimSpace(v.Title),
		URL:       v.URL,
		Thumbnail: v.MainThumb,
		ScrapedAt: now,
	}

	if dur, err := strconv.Atoi(v.Length); err == nil {
		sc.Duration = dur
	}

	sc.Description = strings.TrimSpace(v.Description)

	if v.Models != "" {
		sc.Performers = splitCSV(v.Models)
	}

	if v.Channels != "" {
		sc.Tags = splitCSV(v.Channels)
	}

	if m := thumbDateRe.FindStringSubmatch(v.MainThumb); m != nil {
		if t, err := time.Parse("2006-01-02", m[1]); err == nil {
			sc.Date = t.UTC()
		}
	}

	sc.AddPrice(models.PriceSnapshot{Date: now, IsFree: false})

	return sc
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ", ")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			result = append(result, v)
		}
	}
	return result
}
