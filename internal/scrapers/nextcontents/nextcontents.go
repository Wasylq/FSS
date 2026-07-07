// Package nextcontents scrapes a family of Next.js tour sites (FreakMob Media,
// Deepthroat Sirens, Swallowed) that share one CMS: an mjedge.net thumbnail CDN,
// NATS integration, and a `pageProps.contents` JSON payload served from
// /_next/data/{buildId}/{listPath}.json?page=N. The buildId rotates on redeploy
// and is scraped from the listing page's __NEXT_DATA__ before paginating.
package nextcontents

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

type siteConfig struct {
	SiteID    string
	Studio    string
	Base      string // tour origin, e.g. https://www.freakmobmedia.com
	ListPath  string // JSON/listing route stem: "videos" or "scenes"
	BrandSlug string // model slug for the studio's own brand, dropped from performers
}

var sites = []siteConfig{
	{"freakmob", "FreakMob Media", "https://www.freakmobmedia.com", "videos", "freakmob"},
	{"deepthroatsirens", "Deepthroat Sirens", "https://tour.deepthroatsirens.com", "scenes", ""},
}

type Scraper struct {
	cfg     siteConfig
	client  *http.Client
	matchRe *regexp.Regexp
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func newScraper(cfg siteConfig) *Scraper {
	host := strings.TrimPrefix(cfg.Base, "https://")
	host = strings.TrimPrefix(host, "http://")
	escaped := strings.ReplaceAll(host, ".", `\.`)
	return &Scraper{
		cfg:     cfg,
		client:  httpx.NewClient(30 * time.Second),
		matchRe: regexp.MustCompile(`^https?://` + escaped),
	}
}

func init() {
	for _, cfg := range sites {
		scraper.Register(newScraper(cfg))
	}
}

func (s *Scraper) ID() string { return s.cfg.SiteID }

func (s *Scraper) Patterns() []string {
	host := strings.TrimPrefix(strings.TrimPrefix(s.cfg.Base, "https://"), "http://")
	return []string{host, host + "/" + s.cfg.ListPath}
}

func (s *Scraper) MatchesURL(u string) bool { return s.matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, opts, out)
	return out, nil
}

var buildIDRe = regexp.MustCompile(`"buildId":"([^"]+)"`)

func (s *Scraper) run(ctx context.Context, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	buildID, err := s.fetchBuildID(ctx)
	if err != nil {
		select {
		case out <- scraper.Error(err):
		case <-ctx.Done():
		}
		return
	}
	scraper.Debugf(1, "%s: buildId=%s", s.cfg.SiteID, buildID)

	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, s.cfg.SiteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		contents, err := s.fetchPage(ctx, buildID, page)
		if err != nil {
			return scraper.PageResult{}, err
		}
		scenes := make([]models.Scene, 0, len(contents.Data))
		for _, item := range contents.Data {
			scenes = append(scenes, s.toScene(item, now))
		}
		return scraper.PageResult{
			Scenes: scenes,
			Total:  contents.Total,
			Done:   contents.TotalPages > 0 && page >= contents.TotalPages,
		}, nil
	})
}

func (s *Scraper) fetchBuildID(ctx context.Context) (string, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     s.cfg.Base + "/" + s.cfg.ListPath,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return "", err
	}
	m := buildIDRe.FindSubmatch(body)
	if m == nil {
		return "", fmt.Errorf("%s: buildId not found in listing page", s.cfg.SiteID)
	}
	return string(m[1]), nil
}

func (s *Scraper) fetchPage(ctx context.Context, buildID string, page int) (contentsPage, error) {
	u := fmt.Sprintf("%s/_next/data/%s/%s.json?page=%d", s.cfg.Base, buildID, s.cfg.ListPath, page)
	scraper.Debugf(1, "%s: fetching page %d", s.cfg.SiteID, page)
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     u,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return contentsPage{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	var body nextData
	if err := httpx.DecodeJSON(resp.Body, &body); err != nil {
		return contentsPage{}, fmt.Errorf("decoding %s page %d: %w", s.cfg.SiteID, page, err)
	}
	return body.PageProps.Contents, nil
}

func (s *Scraper) toScene(item contentItem, now time.Time) models.Scene {
	sc := models.Scene{
		ID:          fmt.Sprintf("%d", item.ID),
		SiteID:      s.cfg.SiteID,
		StudioURL:   s.cfg.Base,
		Title:       strings.TrimSpace(item.Title),
		URL:         fmt.Sprintf("%s/scenes/%s", s.cfg.Base, item.Slug),
		Description: strings.TrimSpace(item.Description),
		Studio:      s.cfg.Studio,
		Thumbnail:   item.Thumb,
		Duration:    item.SecondsDuration,
		Tags:        item.Tags,
		Performers:  s.performers(item),
		ScrapedAt:   now,
	}
	if t := parsePublishDate(item.PublishDate); !t.IsZero() {
		sc.Date = t
	}
	if item.ContentPrice > 0 {
		date := sc.Date
		if date.IsZero() {
			date = now
		}
		sc.AddPrice(models.PriceSnapshot{Date: date, Regular: float64(item.ContentPrice)})
	}
	return sc
}

// performers returns the model names with the studio's own brand entry removed.
func (s *Scraper) performers(item contentItem) []string {
	var out []string
	for _, m := range item.ModelsSlugs {
		if s.cfg.BrandSlug != "" && strings.EqualFold(m.Slug, s.cfg.BrandSlug) {
			continue
		}
		name := strings.TrimSpace(m.Name)
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

// parsePublishDate parses the CMS date format "2026/06/13 12:00:00" to UTC.
func parsePublishDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse("2006/01/02 15:04:05", s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

type nextData struct {
	PageProps struct {
		Contents contentsPage `json:"contents"`
	} `json:"pageProps"`
}

type contentsPage struct {
	Total      int           `json:"total"`
	TotalPages int           `json:"total_pages"`
	Data       []contentItem `json:"data"`
}

type contentItem struct {
	ID              int64       `json:"id"`
	Title           string      `json:"title"`
	Slug            string      `json:"slug"`
	PublishDate     string      `json:"publish_date"`
	SecondsDuration int         `json:"seconds_duration"`
	Thumb           string      `json:"thumb"`
	Description     string      `json:"description"`
	ContentPrice    int         `json:"content_price"`
	Tags            []string    `json:"tags"`
	ModelsSlugs     []modelSlug `json:"models_slugs"`
}

type modelSlug struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}
