package clips4sale

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const (
	defaultSiteBase = "https://www.clips4sale.com"
	defaultPageLimit = 24
	// routeKey is the Remix loader data key embedded in the page HTML.
	routeKey = `routes/($lang).studio.$id_.$studioSlug.$`
	// remixMarker is the script prefix before the JSON context object.
	remixMarker = "window.__remixContext = "
)

// Scraper implements scraper.StudioScraper for Clips4Sale.
type Scraper struct {
	client    *http.Client
	siteBase  string
	pageLimit int
}

func New() *Scraper {
	return &Scraper{
		client:    &http.Client{Timeout: 30 * time.Second},
		siteBase:  defaultSiteBase,
		pageLimit: defaultPageLimit,
	}
}

func init() {
	scraper.Register(New())
}

// ---- StudioScraper interface ----

func (s *Scraper) ID() string { return "clips4sale" }

func (s *Scraper) Patterns() []string {
	return []string{
		"clips4sale.com/studio/{studioId}/{studioSlug}",
	}
}

// studioRe matches studio URLs but not individual clip URLs.
// Studio slugs start with a letter; clip IDs are purely numeric.
var studioRe = regexp.MustCompile(`clips4sale\.com/studio/(\d+)/([a-zA-Z][^/?]*)`)

func (s *Scraper) MatchesURL(u string) bool {
	return studioRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	sid, slug, err := studioParams(studioURL)
	if err != nil {
		return nil, err
	}
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, sid, slug, opts, out)
	return out, nil
}

// ---- worker orchestration ----

// run paginates the Clips4Sale studio page HTML. All metadata is embedded in
// window.__remixContext, so no separate API call is needed. KnownIDs are
// skipped per-clip rather than stopping pagination early, because the default
// C4SSort-recommended ordering is not date-ordered.
func (s *Scraper) run(ctx context.Context, studioURL, sid, slug string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)
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
		clips, clipsCount, err := s.fetchPage(ctx, sid, slug, page)
		if err != nil {
			select {
			case out <- scraper.SceneResult{Err: fmt.Errorf("page %d: %w", page, err)}:
			case <-ctx.Done():
			}
			return
		}
		if len(clips) == 0 {
			return
		}
		// After the first page, send a total hint so the consumer can show progress.
		if page == 1 && clipsCount > 0 {
			select {
			case out <- scraper.SceneResult{Total: clipsCount}:
			case <-ctx.Done():
				return
			}
		}
		for _, clip := range clips {
			// KnownIDs are not skipped: C4S uses recommended sort, not date order,
			// so early-stop optimisation cannot be used. All clips are emitted in
			// site order; scrapeIncremental carries price history for known IDs.
			scene, err := toScene(studioURL, s.siteBase, clip, now)
			select {
			case out <- scraper.SceneResult{Scene: scene, Err: err}:
			case <-ctx.Done():
				return
			}
		}
		// C4S may return fewer than pageLimit clips per page even mid-catalogue.
		// Stop only when the page is empty; one extra empty-page request is fine.
	}
}

// ---- page fetch ----

func (s *Scraper) fetchPage(ctx context.Context, studioID, slug string, page int) ([]c4sClip, int, error) {
	u := fmt.Sprintf(
		"%s/studio/%s/%s/Cat0-AllCategories/Page%d/C4SSort-recommended/Limit%d/?onlyClips=true",
		s.siteBase, studioID, slug, page, s.pageLimit,
	)
	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
		"Accept":     "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
	}
	resp, err := get(ctx, s.client, u, headers)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("reading response: %w", err)
	}

	return extractClips(body)
}

// extractClips finds window.__remixContext in page HTML and returns the clips
// embedded in the route loader data, along with the total clip count.
func extractClips(body []byte) ([]c4sClip, int, error) {
	idx := bytes.Index(body, []byte(remixMarker))
	if idx < 0 {
		return nil, 0, fmt.Errorf("remixContext not found in page")
	}

	var rctx remixContext
	if err := json.NewDecoder(bytes.NewReader(body[idx+len(remixMarker):])).Decode(&rctx); err != nil {
		return nil, 0, fmt.Errorf("parsing remixContext: %w", err)
	}

	raw, ok := rctx.State.LoaderData[routeKey]
	if !ok {
		return nil, 0, fmt.Errorf("route key %q not found in loaderData", routeKey)
	}

	var ld loaderData
	if err := json.Unmarshal(raw, &ld); err != nil {
		return nil, 0, fmt.Errorf("parsing loader data: %w", err)
	}

	return ld.Clips, ld.ClipsCount, nil
}

// get performs a GET with up to 3 attempts, backing off 2s then 4s.
func get(ctx context.Context, client *http.Client, url string, headers map[string]string) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}
		return resp, nil
	}
	return nil, lastErr
}

// ---- mapping ----

func toScene(studioURL, siteBase string, clip c4sClip, now time.Time) (models.Scene, error) {
	// Tags: related categories + keywords, deduped.
	seen := make(map[string]bool)
	var tags []string
	for _, rc := range clip.RelatedCategoryLinks {
		if rc.Category != "" && !seen[rc.Category] {
			seen[rc.Category] = true
			tags = append(tags, rc.Category)
		}
	}
	for _, kw := range clip.KeywordLinks {
		if kw.Keyword != "" && !seen[kw.Keyword] {
			seen[kw.Keyword] = true
			tags = append(tags, kw.Keyword)
		}
	}

	performers := make([]string, 0, len(clip.Performers))
	for _, p := range clip.Performers {
		if p.StageName != "" {
			performers = append(performers, p.StageName)
		}
	}

	width, height := parseScreenSize(clip.ScreenSize)

	var categories []string
	if clip.CategoryName != "" {
		categories = []string{clip.CategoryName}
	}

	scene := models.Scene{
		ID:          clip.ClipID,
		SiteID:      "clips4sale",
		StudioURL:   studioURL,
		Title:       clip.Title,
		URL:         siteBase + clip.Link,
		Date:        parseDate(clip.DateDisplay),
		Description: stripHTML(clip.Description),
		Thumbnail:   clip.CDNPreviewLgLink,
		Preview:     clip.CustomPreviewURL,
		Performers:  performers,
		Studio:      clip.StudioTitle,
		Tags:        tags,
		Categories:  categories,
		Duration:    int(clip.TimeMinutes * 60),
		Resolution:  strings.ToUpper(clip.ResolutionText),
		Width:       width,
		Height:      height,
		Format:      strings.ToUpper(clip.Format),
		ScrapedAt:   now,
	}

	discounted := 0.0
	if clip.DiscountedPrice != nil {
		discounted = *clip.DiscountedPrice
	}
	isOnSale := false
	if clip.OnSale != nil {
		isOnSale = *clip.OnSale
	}
	scene.AddPrice(models.PriceSnapshot{
		Date:       now,
		Regular:    clip.Price,
		Discounted: discounted,
		IsOnSale:   isOnSale,
	})

	return scene, nil
}

// ---- helpers ----

func studioParams(u string) (id, slug string, err error) {
	m := studioRe.FindStringSubmatch(u)
	if m == nil {
		return "", "", fmt.Errorf("cannot extract studio params from %q", u)
	}
	return m[1], m[2], nil
}

// parseDate parses Clips4Sale date strings like "1/24/25 10:22 PM".
func parseDate(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse("1/2/06 3:04 PM", s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, "")
	return strings.TrimSpace(html.UnescapeString(s))
}

func parseScreenSize(s string) (width, height int) {
	parts := strings.SplitN(s, "x", 2)
	if len(parts) != 2 {
		return 0, 0
	}
	w, _ := strconv.Atoi(parts[0])
	h, _ := strconv.Atoi(parts[1])
	return w, h
}

// ---- remixContext types ----

type remixContext struct {
	State struct {
		LoaderData map[string]json.RawMessage `json:"loaderData"`
	} `json:"state"`
}

type loaderData struct {
	Clips      []c4sClip `json:"clips"`
	ClipsCount int       `json:"clipsCount"`
	Page       int       `json:"page"`
}

// ---- clip struct ----

type c4sClip struct {
	ClipID               string          `json:"clipId"`
	Title                string          `json:"title"`
	Link                 string          `json:"link"`
	DateDisplay          string          `json:"date_display"`
	Description          string          `json:"description"`
	CDNPreviewLgLink     string          `json:"cdn_previewlg_link"`
	CustomPreviewURL     string          `json:"customPreviewUrl"`
	Performers           []c4sPerformer  `json:"performers"`
	StudioTitle          string          `json:"studioTitle"`
	CategoryName         string          `json:"category_name"`
	RelatedCategoryLinks []c4sRelatedCat `json:"related_category_links"`
	KeywordLinks         []c4sKeyword    `json:"keyword_links"`
	TimeMinutes          float64         `json:"time_minutes"`
	ResolutionText       string          `json:"resolution_text"`
	ScreenSize           string          `json:"screen_size"`
	Format               string          `json:"format"`
	Price                float64         `json:"price"`
	DiscountedPrice      *float64        `json:"discounted_price"`
	OnSale               *bool           `json:"onSale"`
}

type c4sPerformer struct {
	StageName string `json:"stage_name"`
}

type c4sRelatedCat struct {
	Category string `json:"category"`
}

type c4sKeyword struct {
	Keyword string `json:"keyword"`
}
