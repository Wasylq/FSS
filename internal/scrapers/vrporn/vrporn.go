package vrporn

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
	apiBase  = "https://vrporn.com/proxy/api/content/v1"
	siteBase = "https://vrporn.com"
	pageSize = 100
)

type Scraper struct {
	client *http.Client
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func New() *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second)}
}

func init() { scraper.Register(New()) }

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?vrporn\.com/(?:studio|pornstars)/`)

func (s *Scraper) ID() string { return "vrporn" }
func (s *Scraper) Patterns() []string {
	return []string{
		"vrporn.com/studio/{slug}",
		"vrporn.com/pornstars/{slug}",
	}
}
func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	studioSlugRe = regexp.MustCompile(`/studio/([^/?#]+)`)
	modelSlugRe  = regexp.MustCompile(`/pornstars/([^/?#]+)`)
)

type urlMode int

const (
	modeStudio urlMode = iota
	modeModel
)

func resolveURL(studioURL string) (mode urlMode, slug string) {
	if m := studioSlugRe.FindStringSubmatch(studioURL); m != nil {
		return modeStudio, m[1]
	}
	if m := modelSlugRe.FindStringSubmatch(studioURL); m != nil {
		return modeModel, m[1]
	}
	return modeStudio, ""
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	mode, slug := resolveURL(studioURL)
	if slug == "" {
		select {
		case out <- scraper.Error(fmt.Errorf("cannot extract slug from %s", studioURL)):
		case <-ctx.Done():
		}
		return
	}

	var endpoint string
	switch mode {
	case modeStudio:
		endpoint = fmt.Sprintf("%s/videos/studio/%s", apiBase, slug)
		scraper.Debugf(1, "vrporn: scraping studio %s", slug)
	case modeModel:
		endpoint = fmt.Sprintf("%s/videos/model/%s", apiBase, slug)
		scraper.Debugf(1, "vrporn: scraping model %s", slug)
	}

	scraper.Paginate(ctx, opts, "vrporn", out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		u := fmt.Sprintf("%s?page=%d&limit=%d&sort=new", endpoint, page, pageSize)

		var resp apiResponse
		if err := s.fetchJSON(ctx, u, &resp); err != nil {
			return scraper.PageResult{}, err
		}
		if resp.Status.Code != 1 {
			return scraper.PageResult{}, fmt.Errorf("API error: %s", resp.Status.Message)
		}

		now := time.Now().UTC()
		scenes := make([]models.Scene, 0, len(resp.Data.Items))
		for _, item := range resp.Data.Items {
			scenes = append(scenes, toScene(item, studioURL, now))
		}

		return scraper.PageResult{
			Scenes: scenes,
			Total:  resp.Data.Total,
			Done:   page >= resp.Data.Pages,
		}, nil
	})
}

// ---- API types ----

type apiResponse struct {
	Status apiStatus `json:"status"`
	Data   apiData   `json:"data"`
}

type apiStatus struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type apiData struct {
	Pages int       `json:"pages"`
	Total int       `json:"total"`
	Items []apiItem `json:"items"`
}

type apiItem struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	PublishedAt  int64     `json:"publishedAt"`
	Time         int       `json:"time"`
	Models       []string  `json:"models"`
	Studio       apiStudio `json:"studio"`
	PreviewImage apiImage  `json:"previewImage"`
	Likes        int       `json:"likes"`
	Views        int       `json:"views"`
}

type apiStudio struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type apiImage struct {
	Path string `json:"path"`
}

// ---- Scene construction ----

func toScene(item apiItem, studioURL string, now time.Time) models.Scene {
	scene := models.Scene{
		ID:         item.ID,
		SiteID:     "vrporn",
		StudioURL:  studioURL,
		Title:      item.Name,
		URL:        siteBase + "/" + item.Slug + "/",
		Duration:   item.Time,
		Performers: item.Models,
		Studio:     item.Studio.Name,
		Views:      item.Views,
		Likes:      item.Likes,
		ScrapedAt:  now,
	}
	if item.PublishedAt > 0 {
		scene.Date = time.Unix(item.PublishedAt, 0).UTC()
	}
	if item.PreviewImage.Path != "" {
		scene.Thumbnail = item.PreviewImage.Path
	}
	return scene
}

func (s *Scraper) fetchJSON(ctx context.Context, rawURL string, v any) error {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: rawURL,
		Headers: map[string]string{
			"Accept":     "application/json",
			"User-Agent": httpx.UserAgentChrome,
		},
	})
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.DecodeJSON(resp.Body, v)
}
