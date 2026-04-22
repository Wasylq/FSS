package manyvids

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const (
	defaultAPIBase  = "https://api.manyvids.com"
	defaultSiteBase = "https://www.manyvids.com"
)

// Scraper implements scraper.StudioScraper for ManyVids.
type Scraper struct {
	client   *http.Client
	apiBase  string
	siteBase string
}

func New() *Scraper {
	return &Scraper{
		client:   &http.Client{Timeout: 30 * time.Second},
		apiBase:  defaultAPIBase,
		siteBase: defaultSiteBase,
	}
}

func init() {
	scraper.Register(New())
}

// ---- StudioScraper interface ----

func (s *Scraper) ID() string { return "manyvids" }

func (s *Scraper) Patterns() []string {
	return []string{
		"manyvids.com/Profile/{creatorId}/{slug}/Store/Videos",
	}
}

var profileRe = regexp.MustCompile(`manyvids\.com/Profile/(\d+)/[^/]+/Store/Videos`)

func (s *Scraper) MatchesURL(u string) bool {
	return profileRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	cid, err := creatorID(studioURL)
	if err != nil {
		return nil, err
	}
	workers := opts.Workers
	if workers <= 0 {
		workers = 3
	}
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, cid, workers, out)
	return out, nil
}

// ---- internal types ----

type listEntry struct {
	id         string
	previewURL string
}

// ---- worker orchestration ----

func (s *Scraper) run(ctx context.Context, studioURL, cid string, workers int, out chan<- scraper.SceneResult) {
	defer close(out)

	work := make(chan listEntry, workers)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for entry := range work {
				scene, err := s.fetchDetail(ctx, studioURL, entry.id, entry.previewURL)
				select {
				case out <- scraper.SceneResult{Scene: scene, Err: err}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	for page := 1; ; page++ {
		if ctx.Err() != nil {
			break
		}
		entries, totalPages, err := s.fetchPage(ctx, cid, page)
		if err != nil {
			select {
			case out <- scraper.SceneResult{Err: fmt.Errorf("page %d: %w", page, err)}:
			case <-ctx.Done():
			}
			break
		}
		cancelled := false
		for _, e := range entries {
			select {
			case work <- e:
			case <-ctx.Done():
				cancelled = true
			}
			if cancelled {
				break
			}
		}
		if cancelled || page >= totalPages {
			break
		}
	}

	close(work)
	wg.Wait()
}

// ---- API calls ----

func (s *Scraper) fetchPage(ctx context.Context, cid string, page int) ([]listEntry, int, error) {
	u := fmt.Sprintf("%s/store/videos/%s?sort=date&page=%d", s.apiBase, cid, page)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	var lr listResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return nil, 0, fmt.Errorf("decoding list response: %w", err)
	}

	entries := make([]listEntry, len(lr.Data))
	for i, item := range lr.Data {
		entries[i] = listEntry{id: item.ID, previewURL: item.Preview.URL}
	}
	return entries, lr.Pagination.TotalPages, nil
}

func (s *Scraper) fetchDetail(ctx context.Context, studioURL, id, previewURL string) (models.Scene, error) {
	u := fmt.Sprintf("%s/store/video/%s", s.apiBase, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return models.Scene{}, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return models.Scene{}, err
	}
	defer resp.Body.Close()

	var dr detailResponse
	if err := json.NewDecoder(resp.Body).Decode(&dr); err != nil {
		return models.Scene{}, fmt.Errorf("decoding detail for %s: %w", id, err)
	}

	return toScene(studioURL, s.siteBase, dr.Data, previewURL, time.Now().UTC())
}

// ---- mapping ----

func toScene(studioURL, siteBase string, item detailItem, previewURL string, now time.Time) (models.Scene, error) {
	tags := make([]string, len(item.TagList))
	for i, t := range item.TagList {
		tags[i] = t.Label
	}

	regular, _ := strconv.ParseFloat(item.Price.Regular, 64)
	discounted, _ := strconv.ParseFloat(item.Price.DiscountedPrice, 64)

	scene := models.Scene{
		ID:          item.ID,
		SiteID:      "manyvids",
		StudioURL:   studioURL,
		Title:       item.Title,
		URL:         siteBase + item.URL,
		Date:        parseDate(item.LaunchDate),
		Description: html.UnescapeString(item.Description),
		Thumbnail:   item.Screenshot,
		Preview:     previewURL,
		Performers:  []string{item.Model.DisplayName},
		Studio:      item.Model.DisplayName,
		Tags:        tags,
		Duration:    parseDuration(item.VideoDuration),
		Resolution:  item.Resolution,
		Width:       item.Width,
		Height:      item.Height,
		Format:      item.Extension,
		Views:       item.ViewsRaw,
		Likes:       item.LikesRaw,
		Comments:    item.Comments,
		ScrapedAt:   now,
	}

	scene.AddPrice(models.PriceSnapshot{
		Date:            now,
		Regular:         regular,
		Discounted:      discounted,
		IsFree:          item.Price.Free,
		IsOnSale:        item.Price.OnSale,
		DiscountPercent: item.Price.PromoRate,
	})

	return scene, nil
}

// ---- helpers ----

func creatorID(studioURL string) (string, error) {
	m := profileRe.FindStringSubmatch(studioURL)
	if m == nil {
		return "", fmt.Errorf("cannot extract creator ID from %q", studioURL)
	}
	return m[1], nil
}

// parseDuration converts "MM:SS" or "HH:MM:SS" to seconds.
func parseDuration(s string) int {
	parts := strings.Split(s, ":")
	total := 0
	for _, p := range parts {
		n, _ := strconv.Atoi(p)
		total = total*60 + n
	}
	return total
}

// parseDate parses ManyVids API timestamps (RFC3339 with optional milliseconds).
func parseDate(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err == nil {
		return t.UTC()
	}
	t, _ = time.Parse(time.RFC3339, s)
	return t.UTC()
}

// ---- API response types ----

type listResponse struct {
	StatusCode int        `json:"statusCode"`
	Data       []listItem `json:"data"`
	Pagination pagination `json:"pagination"`
}

type listItem struct {
	ID      string `json:"id"`
	Preview struct {
		URL string `json:"url"`
	} `json:"preview"`
}

type pagination struct {
	TotalPages  int `json:"totalPages"`
	CurrentPage int `json:"currentPage"`
	NextPage    int `json:"nextPage"`
}

type detailResponse struct {
	StatusCode int        `json:"statusCode"`
	Data       detailItem `json:"data"`
}

type detailItem struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	LaunchDate    string  `json:"launchDate"`
	VideoDuration string  `json:"videoDuration"`
	Description   string  `json:"description"`
	TagList       []mvTag `json:"tagList"`
	Thumbnail     string  `json:"thumbnail"`
	Screenshot    string  `json:"screenshot"`
	Model         mvModel `json:"model"`
	Resolution    string  `json:"resolution"`
	Width         int     `json:"width"`
	Height        int     `json:"height"`
	Extension     string  `json:"extension"`
	URL           string  `json:"url"`
	ViewsRaw      int     `json:"viewsRaw"`
	LikesRaw      int     `json:"likesRaw"`
	Comments      int     `json:"comments"`
	Price         mvPrice `json:"price"`
}

type mvTag struct {
	Label string `json:"label"`
}

type mvModel struct {
	DisplayName string `json:"displayName"`
}

type mvPrice struct {
	Free            bool   `json:"free"`
	OnSale          bool   `json:"onSale"`
	Regular         string `json:"regular"`
	DiscountedPrice string `json:"discountedPrice"`
	PromoRate       int    `json:"promoRate"`
}
