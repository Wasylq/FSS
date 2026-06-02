// Package kbproductions registers scrapers for KB Productions network sites
// that use the Next.js Paysite.com template with __NEXT_DATA__ JSON.
// Sites using the older IndieBucks template are in the indiebucks package.
package kbproductions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

type siteConfig struct {
	id       string
	domain   string
	studio   string
	matchRe  *regexp.Regexp
	patterns []string
}

var sites = []siteConfig{
	{
		id:       "melinamay",
		domain:   "melina-may.com",
		studio:   "Melina-May",
		matchRe:  regexp.MustCompile(`^https?://(?:www\.)?melina-may\.com`),
		patterns: []string{"melina-may.com", "melina-may.com/videos", "melina-may.com/models/{slug}"},
	},
	{
		id:       "passionpov",
		domain:   "passionpov.com",
		studio:   "PassionPOV",
		matchRe:  regexp.MustCompile(`^https?://(?:www\.)?passionpov\.com`),
		patterns: []string{"passionpov.com", "passionpov.com/videos", "passionpov.com/models/{slug}"},
	},
	{
		id:       "shehergirls",
		domain:   "shehergirls.com",
		studio:   "She Her Girls",
		matchRe:  regexp.MustCompile(`^https?://(?:www\.)?shehergirls\.com`),
		patterns: []string{"shehergirls.com", "shehergirls.com/videos", "shehergirls.com/models/{slug}"},
	},
	{
		id:       "vrallure",
		domain:   "vrallure.com",
		studio:   "VR Allure",
		matchRe:  regexp.MustCompile(`^https?://(?:www\.)?vrallure\.com`),
		patterns: []string{"vrallure.com", "vrallure.com/videos", "vrallure.com/models/{id}-{slug}"},
	},
	{
		id:       "manpuppy",
		domain:   "manpuppy.com",
		studio:   "ManPuppy",
		matchRe:  regexp.MustCompile(`^https?://(?:www\.)?manpuppy\.com`),
		patterns: []string{"manpuppy.com", "manpuppy.com/videos", "manpuppy.com/models/{id}-{slug}"},
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(newScraper(cfg))
	}
}

type siteScraper struct {
	cfg    siteConfig
	client *http.Client
}

func newScraper(cfg siteConfig) *siteScraper {
	return &siteScraper{cfg: cfg, client: httpx.NewClient(30 * time.Second)}
}

var _ scraper.StudioScraper = (*siteScraper)(nil)

func (s *siteScraper) ID() string               { return s.cfg.id }
func (s *siteScraper) Patterns() []string       { return s.cfg.patterns }
func (s *siteScraper) MatchesURL(u string) bool { return s.cfg.matchRe.MatchString(u) }

func (s *siteScraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// --- Next.js data types ---

type nextData struct {
	Props struct {
		PageProps struct {
			Contents contentResponse `json:"contents"`
			Model    *modelData      `json:"model"`
		} `json:"pageProps"`
	} `json:"props"`
}

type contentResponse struct {
	Total      int           `json:"total"`
	TotalPages int           `json:"total_pages"`
	Data       []contentItem `json:"data"`
}

type contentItem struct {
	ID              int         `json:"id"`
	Title           string      `json:"title"`
	Slug            string      `json:"slug"`
	PublishDate     string      `json:"publish_date"`
	SecondsDuration int         `json:"seconds_duration"`
	Thumb           string      `json:"thumb"`
	Models          []string    `json:"models"`
	ModelsSlugs     []modelSlug `json:"models_slugs"`
	Tags            []string    `json:"tags"`
	Description     string      `json:"description"`
	ContentPrice    float64     `json:"content_price"`
	Site            string      `json:"site"`
}

type modelSlug struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type modelData struct {
	Contents contentResponse `json:"contents"`
}

var (
	nextRe  = regexp.MustCompile(`(?s)<script\s+id="__NEXT_DATA__"\s+type="application/json">(.*?)</script>`)
	modelRe = regexp.MustCompile(`/models/([\w-]+)`)
)

// --- runner ---

func (s *siteScraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)
	now := time.Now().UTC()
	base := "https://" + s.cfg.domain
	if u, err := url.Parse(studioURL); err == nil && u.Host != "" {
		base = u.Scheme + "://" + u.Host
	}

	if modelRe.MatchString(studioURL) {
		scraper.Debugf(1, "%s: scraping model page", s.cfg.id)
		s.scrapeModelPage(ctx, studioURL, opts, out, now)
		return
	}

	scraper.Paginate(ctx, opts, s.cfg.id, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := fmt.Sprintf("%s/videos?page=%d", base, page)
		items, total, totalPages, err := s.fetchListing(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}
		scenes := make([]models.Scene, len(items))
		for i, item := range items {
			scenes[i] = s.toScene(item, studioURL, now)
		}
		return scraper.PageResult{
			Scenes: scenes,
			Total:  total,
			Done:   len(items) == 0 || (totalPages > 0 && page >= totalPages),
		}, nil
	})
}

func (s *siteScraper) scrapeModelPage(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult, now time.Time) {
	body, err := s.fetchPage(ctx, studioURL)
	if err != nil {
		select {
		case out <- scraper.Error(fmt.Errorf("model page: %w", err)):
		case <-ctx.Done():
		}
		return
	}

	nd, err := parseNextData(body)
	if err != nil {
		select {
		case out <- scraper.Error(fmt.Errorf("model page: %w", err)):
		case <-ctx.Done():
		}
		return
	}

	var items []contentItem
	if nd.Props.PageProps.Model != nil {
		items = nd.Props.PageProps.Model.Contents.Data
	}
	if len(items) == 0 {
		items = nd.Props.PageProps.Contents.Data
	}

	scraper.Debugf(1, "%s: model page has %d scenes", s.cfg.id, len(items))
	if len(items) > 0 {
		select {
		case out <- scraper.Progress(len(items)):
		case <-ctx.Done():
			return
		}
	}
	for _, item := range items {
		select {
		case out <- scraper.Scene(s.toScene(item, studioURL, now)):
		case <-ctx.Done():
			return
		}
	}
}

func (s *siteScraper) fetchListing(ctx context.Context, pageURL string) ([]contentItem, int, int, error) {
	body, err := s.fetchPage(ctx, pageURL)
	if err != nil {
		return nil, 0, 0, err
	}
	nd, err := parseNextData(body)
	if err != nil {
		return nil, 0, 0, err
	}
	c := nd.Props.PageProps.Contents
	return c.Data, c.Total, c.TotalPages, nil
}

func (s *siteScraper) fetchPage(ctx context.Context, rawURL string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     rawURL,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}

func parseNextData(body []byte) (nextData, error) {
	m := nextRe.FindSubmatch(body)
	if m == nil {
		return nextData{}, fmt.Errorf("__NEXT_DATA__ not found")
	}
	var nd nextData
	if err := json.Unmarshal(m[1], &nd); err != nil {
		return nextData{}, fmt.Errorf("parsing __NEXT_DATA__: %w", err)
	}
	return nd, nil
}

func (s *siteScraper) toScene(item contentItem, studioURL string, now time.Time) models.Scene {
	var date time.Time
	if item.PublishDate != "" {
		date, _ = time.Parse("2006/01/02 15:04:05", item.PublishDate)
		date = date.UTC()
	}

	var performers []string
	for _, ms := range item.ModelsSlugs {
		if ms.Name != "" {
			performers = append(performers, ms.Name)
		}
	}
	if len(performers) == 0 {
		performers = item.Models
	}

	studio := s.cfg.studio
	if item.Site != "" {
		studio = item.Site
	}

	scene := models.Scene{
		ID:          strconv.Itoa(item.ID),
		SiteID:      s.cfg.id,
		StudioURL:   studioURL,
		Title:       item.Title,
		URL:         fmt.Sprintf("https://%s/videos/%s", s.cfg.domain, item.Slug),
		Thumbnail:   item.Thumb,
		Duration:    item.SecondsDuration,
		Date:        date,
		Description: item.Description,
		Tags:        item.Tags,
		Performers:  performers,
		Studio:      studio,
		ScrapedAt:   now,
	}
	if item.ContentPrice > 0 {
		scene.AddPrice(models.PriceSnapshot{
			Date:    now,
			Regular: item.ContentPrice,
		})
	}
	return scene
}
