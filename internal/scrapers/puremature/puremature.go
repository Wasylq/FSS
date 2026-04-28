package puremature

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
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
	matchRe = regexp.MustCompile(`^https?://(?:www\.)?puremature\.com`)
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

		pageURL := fmt.Sprintf("%s%spage=%d&per_page=%d",
			baseURL, querySep(baseURL), page, pageSize)

		result, err := s.fetch(ctx, pageURL)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		if len(result.Items) == 0 {
			return
		}

		if page == 1 && result.Pagination.TotalItems > 0 {
			select {
			case out <- scraper.Progress(result.Pagination.TotalItems):
			case <-ctx.Done():
				return
			}
		}

		for _, item := range result.Items {
			id := strconv.Itoa(item.ID)

			if len(opts.KnownIDs) > 0 && opts.KnownIDs[id] {
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
				return
			}

			scene := itemToScene(item, studioURL, now)
			select {
			case out <- scraper.Scene(scene):
			case <-ctx.Done():
				return
			}
		}

		if result.Pagination.NextPage == nil {
			return
		}
	}
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
		Headers: map[string]string{
			"x-site":     "puremature.com",
			"User-Agent": httpx.UserAgentFirefox,
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
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
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05Z", s)
		if err != nil {
			return time.Time{}
		}
	}
	return t.UTC()
}
