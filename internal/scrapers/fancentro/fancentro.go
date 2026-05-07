package fancentro

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
	"github.com/Wasylq/FSS/scraper"
)

const (
	defaultBase = "https://fancentro.com"
	siteID      = "fancentro"
	perPage     = 24
)

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?fancentro\.com/([a-zA-Z0-9_-]+)/?(?:\?.*)?$`)

type Scraper struct {
	client *http.Client
	base   string
}

func New() *Scraper {
	return &Scraper{
		client: httpx.NewClient(30 * time.Second),
		base:   defaultBase,
	}
}

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string               { return siteID }
func (s *Scraper) Patterns() []string       { return []string{"fancentro.com/{model}"} }
func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func slugFromURL(u string) string {
	// Strip query string, then take the last non-empty path segment.
	if i := strings.IndexByte(u, '?'); i >= 0 {
		u = u[:i]
	}
	u = strings.TrimRight(u, "/")
	if i := strings.LastIndexByte(u, '/'); i >= 0 {
		return u[i+1:]
	}
	return ""
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	slug := slugFromURL(studioURL)
	if slug == "" {
		send(ctx, out, scraper.Error(fmt.Errorf("cannot extract model slug from %s", studioURL)))
		return
	}

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

		items, total, lastPage, err := s.fetchPage(ctx, slug, page)
		if err != nil {
			send(ctx, out, scraper.Error(err))
			return
		}

		if len(items) == 0 {
			return
		}

		if page == 1 && total > 0 {
			if !send(ctx, out, scraper.Progress(total)) {
				return
			}
		}

		now := time.Now().UTC()
		for _, item := range items {
			id := strconv.Itoa(item.ID)

			if opts.KnownIDs[id] {
				send(ctx, out, scraper.StoppedEarly())
				return
			}

			scene := toScene(item, studioURL, s.base, now)
			if !send(ctx, out, scraper.Scene(scene)) {
				return
			}
		}

		if page >= lastPage {
			return
		}
	}
}

func send(ctx context.Context, ch chan<- scraper.SceneResult, r scraper.SceneResult) bool {
	select {
	case ch <- r:
		return true
	case <-ctx.Done():
		return false
	}
}

// API types.

type apiResponse struct {
	Success    bool      `json:"success"`
	Data       []apiClip `json:"data"`
	TotalItems int       `json:"total_items"`
	LastPage   int       `json:"last_page"`
}

type apiClip struct {
	ID          int            `json:"id"`
	Title       string         `json:"title"`
	Duration    string         `json:"duration"`
	PublishedAt int64          `json:"publishedAt"`
	Link        string         `json:"link"`
	Tags        []apiTag       `json:"tags"`
	Price       *apiPrice      `json:"price"`
	Model       *apiModel      `json:"model"`
	Thumbnails  *apiThumbnails `json:"thumbnails"`
}

type apiTag struct {
	Name string `json:"name"`
}

type apiModel struct {
	StageName string `json:"stageName"`
}

type apiThumbnails struct {
	Thumbnail *apiThumb `json:"thumbnail"`
}

type apiThumb struct {
	Src string `json:"src"`
}

type apiPrice struct {
	OriginalPrice  float64 `json:"originalPrice"`
	DiscountPrice  float64 `json:"discountPrice"`
	HasDiscount    bool    `json:"hasDiscount"`
	Currency       string  `json:"currency"`
	IsFree         bool    `json:"isFree"`
	DiscountAmount int     `json:"discountAmount"`
}

func (s *Scraper) fetchPage(ctx context.Context, slug string, page int) ([]apiClip, int, int, error) {
	u := fmt.Sprintf("%s/api/content/content?alias=%s&type=video&page=%d&page_size=%d",
		s.base, slug, page, perPage)

	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: u,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentChrome,
			"Accept":     "application/json",
		},
	})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("fetch page %d: %w", page, err)
	}
	defer func() { _ = resp.Body.Close() }()

	var ar apiResponse
	if err := httpx.DecodeJSON(resp.Body, &ar); err != nil {
		return nil, 0, 0, fmt.Errorf("parse page %d: %w", page, err)
	}

	return ar.Data, ar.TotalItems, ar.LastPage, nil
}

func parseDuration(s string) int {
	parts := strings.Split(s, ":")
	switch len(parts) {
	case 2:
		m, _ := strconv.Atoi(parts[0])
		sec, _ := strconv.Atoi(parts[1])
		return m*60 + sec
	case 3:
		h, _ := strconv.Atoi(parts[0])
		m, _ := strconv.Atoi(parts[1])
		sec, _ := strconv.Atoi(parts[2])
		return h*3600 + m*60 + sec
	}
	return 0
}

func toScene(clip apiClip, studioURL, base string, now time.Time) models.Scene {
	id := strconv.Itoa(clip.ID)
	sceneURL := base + clip.Link

	sc := models.Scene{
		ID:        id,
		SiteID:    siteID,
		StudioURL: studioURL,
		Title:     clip.Title,
		URL:       sceneURL,
		Duration:  parseDuration(clip.Duration),
		ScrapedAt: now,
	}

	if clip.PublishedAt > 0 {
		sc.Date = time.Unix(clip.PublishedAt, 0).UTC()
	}

	if clip.Model != nil && clip.Model.StageName != "" {
		sc.Studio = clip.Model.StageName
		sc.Performers = []string{clip.Model.StageName}
	}

	if clip.Thumbnails != nil && clip.Thumbnails.Thumbnail != nil {
		sc.Thumbnail = clip.Thumbnails.Thumbnail.Src
	}

	if clip.Tags != nil {
		tags := make([]string, 0, len(clip.Tags))
		for _, t := range clip.Tags {
			if t.Name != "" {
				tags = append(tags, t.Name)
			}
		}
		sc.Tags = tags
	}

	if clip.Price != nil {
		ps := models.PriceSnapshot{
			Date:    now,
			Regular: clip.Price.OriginalPrice,
			IsFree:  clip.Price.IsFree,
		}
		if clip.Price.HasDiscount && clip.Price.DiscountPrice < clip.Price.OriginalPrice {
			ps.IsOnSale = true
			ps.Discounted = clip.Price.DiscountPrice
			ps.DiscountPercent = clip.Price.DiscountAmount
		}
		sc.AddPrice(ps)
	}

	return sc
}
