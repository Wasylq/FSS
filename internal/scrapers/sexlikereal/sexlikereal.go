package sexlikereal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const (
	apiBase  = "https://api.sexlikereal.com"
	siteBase = "https://www.sexlikereal.com"
	perPage  = 36
)

type Scraper struct {
	client     *http.Client
	apiBaseURL string
}

func New() *Scraper {
	return &Scraper{
		client:     httpx.NewClient(30 * time.Second),
		apiBaseURL: apiBase,
	}
}

func init() {
	scraper.Register(New())
}

func (s *Scraper) ID() string { return "sexlikereal" }

func (s *Scraper) Patterns() []string {
	return []string{
		"sexlikereal.com",
		"sexlikereal.com/scenes",
		"sexlikereal.com/studios/{slug}-{id}",
		"sexlikereal.com/pornstars/{slug}-{id}",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?sexlikereal\.com(?:/|$)`)

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

type filterMode int

const (
	filterAll filterMode = iota
	filterStudio
	filterModel
)

var (
	studioRe = regexp.MustCompile(`/studios/[^/]+-(\d+)`)
	modelRe  = regexp.MustCompile(`/pornstars/[^/]+-(\d+)`)
)

func resolveFilter(studioURL string) (filterMode, string) {
	if m := studioRe.FindStringSubmatch(studioURL); m != nil {
		return filterStudio, m[1]
	}
	if m := modelRe.FindStringSubmatch(studioURL); m != nil {
		return filterModel, m[1]
	}
	return filterAll, ""
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	mode, filterID := resolveFilter(studioURL)

	workers := opts.Workers
	if workers <= 0 {
		workers = 4
	}

	work := make(chan apiScene)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range work {
				scene, err := s.fetchAndBuild(ctx, item, studioURL, opts.Delay)
				if err != nil {
					select {
					case out <- scraper.Error(err):
					case <-ctx.Done():
						return
					}
					continue
				}
				select {
				case out <- scraper.Scene(scene):
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(work)

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

			params := url.Values{
				"page":    {strconv.Itoa(page)},
				"perPage": {strconv.Itoa(perPage)},
				"sort":    {"recent"},
			}
			switch mode {
			case filterStudio:
				params.Set("studios", filterID)
			case filterModel:
				params.Set("models", filterID)
			}

			apiURL := s.apiBaseURL + "/v3/scenes?" + params.Encode()
			var resp listResponse
			if err := s.fetchJSON(ctx, apiURL, &resp); err != nil {
				select {
				case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
				case <-ctx.Done():
				}
				return
			}

			if page == 1 && resp.Meta.Pagination.TotalCount > 0 {
				select {
				case out <- scraper.Progress(resp.Meta.Pagination.TotalCount):
				case <-ctx.Done():
					return
				}
			}

			if len(resp.Data) == 0 {
				return
			}

			for _, item := range resp.Data {
				id := strconv.Itoa(item.ID)
				if opts.KnownIDs[id] {
					select {
					case out <- scraper.StoppedEarly():
					case <-ctx.Done():
					}
					return
				}
				select {
				case work <- item:
				case <-ctx.Done():
					return
				}
			}

			if page >= resp.Meta.Pagination.TotalPages {
				return
			}
		}
	}()

	wg.Wait()
}

func (s *Scraper) fetchAndBuild(ctx context.Context, item apiScene, studioURL string, delay time.Duration) (models.Scene, error) {
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return models.Scene{}, ctx.Err()
		}
	}

	apiURL := s.apiBaseURL + "/v3/scenes/" + strconv.Itoa(item.ID)
	var resp detailResponse
	if err := s.fetchJSON(ctx, apiURL, &resp); err != nil {
		return models.Scene{}, fmt.Errorf("detail %d: %w", item.ID, err)
	}

	return toScene(item, resp.Data, studioURL), nil
}

func toScene(item apiScene, detail detailData, studioURL string) models.Scene {
	sc := models.Scene{
		ID:          strconv.Itoa(item.ID),
		SiteID:      "sexlikereal",
		StudioURL:   studioURL,
		Title:       item.Title,
		URL:         siteBase + "/scenes/" + item.Label,
		Thumbnail:   item.ThumbnailURL,
		Description: strings.TrimSpace(item.Description),
		Duration:    item.FullVideoLength,
		ScrapedAt:   time.Now().UTC(),
	}

	if item.Date > 0 {
		sc.Date = time.Unix(int64(item.Date), 0).UTC()
	}

	if item.Studio.Name != "" {
		sc.Studio = item.Studio.Name
	}

	for _, a := range item.Actors {
		sc.Performers = append(sc.Performers, a.Name)
	}

	for _, c := range detail.Categories {
		sc.Tags = append(sc.Tags, c.Name)
	}

	if detail.Price.Amount > 0 {
		sc.AddPrice(models.PriceSnapshot{
			Date:    time.Now().UTC(),
			Regular: detail.Price.Amount,
		})
	}

	return sc
}

func (s *Scraper) fetchJSON(ctx context.Context, u string, v any) error {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: u,
		Headers: func() map[string]string {
			h := httpx.BrowserHeaders(httpx.UserAgentFirefox)
			h["Client-Type"] = "web"
			h["Project"] = "1"
			return h
		}(),
	})
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.DecodeJSON(resp.Body, v)
}

type listResponse struct {
	Data []apiScene `json:"data"`
	Meta struct {
		Pagination struct {
			Page       int `json:"page"`
			PerPage    int `json:"perPage"`
			TotalCount int `json:"totalCount"`
			TotalPages int `json:"totalPages"`
		} `json:"pagination"`
	} `json:"meta"`
}

type detailResponse struct {
	Data detailData `json:"data"`
}

type detailData struct {
	Categories []apiCat `json:"categories"`
	Price      apiPrice `json:"price"`
}

type apiScene struct {
	ID              int        `json:"id"`
	Title           string     `json:"title"`
	Label           string     `json:"label"`
	Description     string     `json:"description"`
	Date            int        `json:"date"`
	FullVideoLength int        `json:"fullVideoLength"`
	ThumbnailURL    string     `json:"thumbnailUrl"`
	Studio          apiStudio  `json:"studio"`
	Actors          []apiActor `json:"actors"`
}

type apiStudio struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Label string `json:"label"`
}

type apiActor struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Label string `json:"label"`
}

type apiCat struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Label string `json:"label"`
}

type apiPrice struct {
	Amount float64 `json:"amount"`
}
