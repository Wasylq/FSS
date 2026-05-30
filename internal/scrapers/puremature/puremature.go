package puremature

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/parseutil"
	"github.com/Wasylq/FSS/scraper"
)

const (
	siteBase = "https://puremature.com"
	pageSize = 50
)

type Scraper struct {
	client *http.Client
}

func New() *Scraper {
	return &Scraper{
		client: httpx.NewClient(30 * time.Second),
	}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() {
	scraper.Register(New())
}

func (s *Scraper) ID() string { return "puremature" }

func (s *Scraper) Patterns() []string {
	return []string{
		"puremature.com",
		"puremature.com/models/{slug}",
	}
}

var (
	matchRe = regexp.MustCompile(`^https?://(?:www\.)?puremature\.com(?:/|$)`)
	modelRe = regexp.MustCompile(`/models/([^/?#]+)`)
)

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	var baseURL string
	if m := modelRe.FindStringSubmatch(studioURL); m != nil {
		baseURL = fmt.Sprintf("%s/api/actors/%s/releases", siteBase, m[1])
	} else {
		baseURL = siteBase + "/api/releases?sort=latest"
	}

	s.runWithBase(ctx, baseURL, studioURL, opts, out)
}

func (s *Scraper) runWithBase(ctx context.Context, baseURL string, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	now := time.Now().UTC()

	scraper.Paginate(ctx, opts, "puremature", out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := fmt.Sprintf("%s%spage=%d&per_page=%d",
			baseURL, querySep(baseURL), page, pageSize)

		result, err := s.fetch(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}

		scenes := make([]models.Scene, len(result.Items))
		for i, item := range result.Items {
			scenes[i] = itemToScene(item, studioURL, now)
		}
		return scraper.PageResult{
			Scenes: scenes,
			Total:  result.Pagination.TotalItems,
			Done:   result.Pagination.NextPage == nil,
		}, nil
	})
}

func querySep(u string) string {
	if strings.Contains(u, "?") {
		return "&"
	}
	return "?"
}

func (s *Scraper) fetch(ctx context.Context, url string) (*apiResponse, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: url,
		Headers: func() map[string]string {
			h := httpx.BrowserHeaders(httpx.UserAgentFirefox)
			h["x-site"] = "puremature.com"
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

type apiResponse struct {
	Items      []apiScene `json:"items"`
	Pagination struct {
		TotalItems int     `json:"totalItems"`
		TotalPages int     `json:"totalPages"`
		NextPage   *string `json:"nextPage"`
	} `json:"pagination"`
}

type apiScene struct {
	ID              int        `json:"id"`
	CachedSlug      string     `json:"cachedSlug"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	ReleasedAt      string     `json:"releasedAt"`
	PosterURL       string     `json:"posterUrl"`
	ThumbURL        string     `json:"thumbUrl"`
	ThumbVideoURL   string     `json:"thumbVideoUrl"`
	TrailerURL      string     `json:"trailerUrl"`
	Tags            []string   `json:"tags"`
	Actors          []apiActor `json:"actors"`
	Sponsor         apiSponsor `json:"sponsor"`
	RatingsDecimal  float64    `json:"ratingsDecimal"`
	DownloadOptions []struct {
		Quality string `json:"quality"`
	} `json:"downloadOptions"`
}

type apiActor struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	CachedSlug string `json:"cached_slug"`
	Gender     string `json:"gender"`
}

type apiSponsor struct {
	Name       string `json:"name"`
	CachedSlug string `json:"cachedSlug"`
}

func itemToScene(item apiScene, studioURL string, now time.Time) models.Scene {
	var performers []string
	for _, a := range item.Actors {
		if a.Name != "" {
			performers = append(performers, a.Name)
		}
	}

	tags := make([]string, 0, len(item.Tags))
	for _, t := range item.Tags {
		tag := strings.ReplaceAll(t, "_", " ")
		tag = strings.TrimSpace(tag)
		if tag != "" {
			tags = append(tags, tag)
		}
	}

	var resolution string
	var width int
	for _, opt := range item.DownloadOptions {
		q, _ := strconv.Atoi(opt.Quality)
		if q > width {
			width = q
		}
	}
	if width >= 2160 {
		resolution = "4K"
	} else if width >= 1080 {
		resolution = "1080p"
	} else if width >= 720 {
		resolution = "720p"
	}

	preview := item.TrailerURL
	if preview == "" {
		preview = item.ThumbVideoURL
	}

	scene := models.Scene{
		ID:          strconv.Itoa(item.ID),
		SiteID:      "puremature",
		StudioURL:   studioURL,
		Title:       item.Title,
		URL:         siteBase + "/video/" + item.CachedSlug,
		Thumbnail:   item.PosterURL,
		Preview:     preview,
		Description: strings.TrimSpace(item.Description),
		Performers:  performers,
		Tags:        tags,
		Date:        parseDate(item.ReleasedAt),
		Studio:      "Pure Mature",
		Resolution:  resolution,
		Height:      width,
		ScrapedAt:   now,
	}
	scene.AddPrice(models.PriceSnapshot{Date: now, IsFree: false})
	return scene
}

func parseDate(s string) time.Time {
	t, _ := parseutil.TryParseDate(s, time.RFC3339, "2006-01-02T15:04:05Z")
	return t.UTC()
}
