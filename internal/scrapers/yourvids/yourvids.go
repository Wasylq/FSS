package yourvids

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

type Scraper struct {
	client  *http.Client
	apiBase string
}

func New() *Scraper {
	return &Scraper{
		client:  httpx.NewClient(30 * time.Second),
		apiBase: "https://yourvids.com",
	}
}

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return "yourvids" }

func (s *Scraper) Patterns() []string {
	return []string{"yourvids.com/creators/{slug}"}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?yourvids\.com/creators/[\w-]+`)
var slugRe = regexp.MustCompile(`/creators/([\w-]+)`)

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	m := slugRe.FindStringSubmatch(studioURL)
	if m == nil {
		return nil, fmt.Errorf("cannot extract creator slug from %q", studioURL)
	}
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, m[1], opts, out)
	return out, nil
}

// ---- API types ----

type apiResponse struct {
	Success bool    `json:"success"`
	Data    apiData `json:"data"`
}

type apiData struct {
	Videos     []apiVideo    `json:"videos"`
	Pagination apiPagination `json:"pagination"`
}

type apiPagination struct {
	CurrentPage int  `json:"current_page"`
	PerPage     int  `json:"per_page"`
	Total       int  `json:"total"`
	TotalPages  int  `json:"total_pages"`
	HasMore     bool `json:"has_more"`
}

type apiVideo struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	Thumbnail     string  `json:"thumbnail"`
	PreviewURL    string  `json:"preview_url"`
	CreatorName   string  `json:"creator_name"`
	Duration      string  `json:"duration"`
	Views         int     `json:"views"`
	Likes         int     `json:"likes"`
	Price         string  `json:"price"`
	OriginalPrice json.RawMessage `json:"original_price"`
	VideoURL      string  `json:"video_url"`
	CreatorURL    string  `json:"creator_url"`
	IsOnSale      bool    `json:"is_on_sale"`
	IsFree        bool    `json:"is_free"`
	IsHD          bool    `json:"is_hd"`
	Is4K          bool    `json:"is_4k"`
	IsAudio       bool    `json:"is_audio"`
	CreatedAt     string  `json:"created_at"`
}

// ---- runner ----

func (s *Scraper) run(ctx context.Context, studioURL, slug string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	workers := opts.Workers
	if workers <= 0 {
		workers = 3
	}

	var allVideos []apiVideo
	stoppedEarly := false

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

		apiURL := fmt.Sprintf("%s/api/creators/%s/videos?page=%d&sort=newest", s.apiBase, slug, page)
		resp, err := s.fetchAPI(ctx, apiURL)
		if err != nil {
			select {
			case out <- scraper.SceneResult{Err: fmt.Errorf("page %d: %w", page, err)}:
			case <-ctx.Done():
			}
			return
		}

		if page == 1 && resp.Data.Pagination.Total > 0 {
			select {
			case out <- scraper.SceneResult{Total: resp.Data.Pagination.Total}:
			case <-ctx.Done():
				return
			}
		}

		hitKnown := false
		for _, v := range resp.Data.Videos {
			id := strconv.Itoa(v.ID)
			if len(opts.KnownIDs) > 0 && opts.KnownIDs[id] {
				hitKnown = true
				stoppedEarly = true
				break
			}
			allVideos = append(allVideos, v)
		}

		if hitKnown || !resp.Data.Pagination.HasMore {
			break
		}
	}

	type detailResult struct {
		video       apiVideo
		description string
		tags        []string
	}

	work := make(chan apiVideo, workers)
	results := make(chan detailResult, workers)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for v := range work {
				if ctx.Err() != nil {
					return
				}
				if opts.Delay > 0 {
					select {
					case <-time.After(opts.Delay):
					case <-ctx.Done():
						return
					}
				}
				desc, tags := s.fetchDetail(ctx, v.VideoURL)
				select {
				case results <- detailResult{video: v, description: desc, tags: tags}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		for _, v := range allVideos {
			select {
			case work <- v:
			case <-ctx.Done():
			}
			if ctx.Err() != nil {
				break
			}
		}
		close(work)
		wg.Wait()
		close(results)
	}()

	now := time.Now().UTC()
	for dr := range results {
		scene := toScene(studioURL, dr.video, dr.description, dr.tags, now)
		select {
		case out <- scraper.SceneResult{Scene: scene}:
		case <-ctx.Done():
			return
		}
	}

	if stoppedEarly {
		select {
		case out <- scraper.SceneResult{StoppedEarly: true}:
		case <-ctx.Done():
		}
	}
}

// ---- API fetch ----

func (s *Scraper) fetchAPI(ctx context.Context, apiURL string) (*apiResponse, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: apiURL,
		Headers: map[string]string{
			"User-Agent":       httpx.UserAgentFirefox,
			"Accept":           "application/json",
			"X-Requested-With": "XMLHttpRequest",
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var ar apiResponse
	if err := json.Unmarshal(body, &ar); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}
	if !ar.Success {
		return nil, fmt.Errorf("API returned success=false")
	}
	return &ar, nil
}

// ---- detail page ----

var (
	dataTagRe   = regexp.MustCompile(`data-tag="([\w-]+)"`)
	descBlockRe = regexp.MustCompile(`(?s)<div[^>]*class="rich-text-content[^"]*"[^>]*>\s*(.*?)\s*</div>`)
)

func (s *Scraper) fetchDetail(ctx context.Context, videoURL string) (description string, tags []string) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: videoURL,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentFirefox,
			"Accept":     "text/html",
		},
	})
	if err != nil {
		return "", nil
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil
	}

	tagMatches := dataTagRe.FindAllSubmatch(body, -1)
	seen := make(map[string]bool)
	for _, m := range tagMatches {
		tag := titleCase(strings.ReplaceAll(string(m[1]), "-", " "))
		if !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
		}
	}

	descMatches := descBlockRe.FindAllSubmatch(body, -1)
	var longest string
	for _, m := range descMatches {
		raw := string(m[1])
		if len(raw) > len(longest) {
			longest = raw
		}
	}
	if longest != "" {
		description = cleanHTML(longest)
	}

	return description, tags
}

// ---- conversion ----

func toScene(studioURL string, v apiVideo, description string, tags []string, now time.Time) models.Scene {
	scene := models.Scene{
		ID:          strconv.Itoa(v.ID),
		SiteID:      "yourvids",
		StudioURL:   studioURL,
		Title:       v.Title,
		URL:         v.VideoURL,
		Thumbnail:   v.Thumbnail,
		Preview:     v.PreviewURL,
		Duration:    parseDuration(v.Duration),
		Views:       v.Views,
		Likes:       v.Likes,
		Performers:  []string{v.CreatorName},
		Studio:      v.CreatorName,
		Description: description,
		Tags:        tags,
		ScrapedAt:   now,
	}

	if v.Is4K {
		scene.Width = 3840
		scene.Height = 2160
		scene.Resolution = "4K"
	} else if v.IsHD {
		scene.Width = 1920
		scene.Height = 1080
		scene.Resolution = "1080p"
	}

	if t, err := time.Parse("2006-01-02 15:04:05", v.CreatedAt); err == nil {
		scene.Date = t.UTC()
	}

	price, _ := strconv.ParseFloat(v.Price, 64)
	snap := models.PriceSnapshot{
		Date:   now,
		IsFree: v.IsFree,
	}
	if v.IsOnSale && len(v.OriginalPrice) > 0 && string(v.OriginalPrice) != "null" {
		orig := parseOriginalPrice(v.OriginalPrice)
		snap.Regular = orig
		snap.Discounted = price
		snap.IsOnSale = true
		if orig > 0 {
			snap.DiscountPercent = int((1 - price/orig) * 100)
		}
	} else {
		snap.Regular = price
	}
	scene.AddPrice(snap)

	return scene
}

// ---- helpers ----

func parseOriginalPrice(raw json.RawMessage) float64 {
	var f float64
	if json.Unmarshal(raw, &f) == nil {
		return f
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		f, _ = strconv.ParseFloat(s, 64)
		return f
	}
	return 0
}

func parseDuration(s string) int {
	parts := strings.Split(s, ":")
	total := 0
	for _, p := range parts {
		n, _ := strconv.Atoi(p)
		total = total*60 + n
	}
	return total
}

func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

func cleanHTML(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = strings.ReplaceAll(s, "</p>", "\n")
	s = htmlTagRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	lines := strings.Split(s, "\n")
	var trimmed []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			trimmed = append(trimmed, l)
		}
	}
	return strings.Join(trimmed, "\n")
}
