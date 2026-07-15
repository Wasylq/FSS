// Package indiebucks registers scrapers for KB Productions network sites
// using the IndieBucks "Hollywood" template with per-card JSON-LD on listing pages.
package indiebucks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
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
		id:       "boyssmoking",
		domain:   "boys-smoking.com",
		studio:   "Boys-Smoking",
		matchRe:  regexp.MustCompile(`^https?://(?:www\.)?boys-smoking\.com`),
		patterns: []string{"boys-smoking.com", "boys-smoking.com/videos"},
	},
	{
		id:       "boyspissing",
		domain:   "boys-pissing.com",
		studio:   "Boys Pissing",
		matchRe:  regexp.MustCompile(`^https?://(?:www\.)?boys-pissing\.com`),
		patterns: []string{"boys-pissing.com", "boys-pissing.com/videos"},
	},
	{
		id:       "boundmusclejocks",
		domain:   "boundmusclejocks.com",
		studio:   "BoundMuscleJocks",
		matchRe:  regexp.MustCompile(`^https?://(?:www\.)?boundmusclejocks\.com`),
		patterns: []string{"boundmusclejocks.com", "boundmusclejocks.com/videos"},
	},

	// BoyNapped group — same YPP/Journey backend and the same "Hollywood"
	// /videos template with per-card JSON-LD. Grouped by the shared
	// secured.westbill.com/contact/ib/boyn/ support bucket. BoyNapped itself
	// runs a different theme and has its own scraper.
	{
		id:       "badboybondage",
		domain:   "badboybondage.com",
		studio:   "Bad Boy Bondage",
		matchRe:  regexp.MustCompile(`^https?://(?:www\.)?badboybondage\.com`),
		patterns: []string{"badboybondage.com", "badboybondage.com/videos"},
	},
	{
		id:       "badboysbootcamp",
		domain:   "badboysbootcamp.com",
		studio:   "Bad Boys Bootcamp",
		matchRe:  regexp.MustCompile(`^https?://(?:www\.)?badboysbootcamp\.com`),
		patterns: []string{"badboysbootcamp.com", "badboysbootcamp.com/videos"},
	},
	{
		id:       "daddysbondageboys",
		domain:   "daddysbondageboys.com",
		studio:   "Daddys Bondage Boys",
		matchRe:  regexp.MustCompile(`^https?://(?:www\.)?daddysbondageboys\.com`),
		patterns: []string{"daddysbondageboys.com", "daddysbondageboys.com/videos"},
	},
	{
		id:       "undietwinks",
		domain:   "undietwinks.com",
		studio:   "Undie Twinks",
		matchRe:  regexp.MustCompile(`^https?://(?:www\.)?undietwinks\.com`),
		patterns: []string{"undietwinks.com", "undietwinks.com/videos"},
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

// --- JSON-LD types ---

type movieLD struct {
	Type          string    `json:"@type"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Image         string    `json:"image"`
	DatePublished string    `json:"datePublished"`
	URL           string    `json:"url"`
	Actors        []actorLD `json:"actors"`
}

type actorLD struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

var (
	jsonLDRe   = regexp.MustCompile(`(?s)<script\s+type="application/ld\+json">\s*(\{.*?\})\s*</script>`)
	pageNumRe  = regexp.MustCompile(`[?&]page=(\d+)`)
	lastLinkRe = regexp.MustCompile(`<a\s+href="([^"]*\?page=\d+)"[^>]*class="one-page-link"[^>]*>(?:Last|&raquo;)</a>`)
)

// --- runner ---

func (s *siteScraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)
	now := time.Now().UTC()
	base := "https://" + s.cfg.domain
	if u, err := url.Parse(studioURL); err == nil && u.Host != "" {
		base = u.Scheme + "://" + u.Host
	}

	scraper.Paginate(ctx, opts, s.cfg.id, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := fmt.Sprintf("%s/videos?page=%d&sort=newest", base, page)
		movies, lastPage, err := s.fetchListing(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}
		scenes := make([]models.Scene, len(movies))
		for i, m := range movies {
			scenes[i] = s.toScene(m, studioURL, now)
		}
		return scraper.PageResult{
			Scenes: scenes,
			Done:   len(movies) == 0 || (lastPage > 0 && page >= lastPage),
		}, nil
	})
}

func (s *siteScraper) fetchListing(ctx context.Context, rawURL string) ([]movieLD, int, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     rawURL,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	var movies []movieLD
	for _, m := range jsonLDRe.FindAllSubmatch(body, -1) {
		var ld movieLD
		if err := json.Unmarshal(m[1], &ld); err != nil {
			continue
		}
		if ld.Type == "Movie" && ld.URL != "" {
			movies = append(movies, ld)
		}
	}

	lastPage := 0
	if m := lastLinkRe.FindSubmatch(body); m != nil {
		if pm := pageNumRe.FindSubmatch(m[1]); pm != nil {
			lastPage, _ = strconv.Atoi(string(pm[1]))
		}
	}

	return movies, lastPage, nil
}

func (s *siteScraper) toScene(m movieLD, studioURL string, now time.Time) models.Scene {
	slug := m.URL
	if i := strings.LastIndex(slug, "/"); i >= 0 {
		slug = slug[i+1:]
	}

	var date time.Time
	if m.DatePublished != "" {
		date, _ = time.Parse("2006-01-02 15:04:05", m.DatePublished)
		date = date.UTC()
	}

	var performers []string
	for _, a := range m.Actors {
		if a.Name != "" {
			performers = append(performers, a.Name)
		}
	}

	thumb := m.Image
	if strings.HasPrefix(thumb, "//") {
		thumb = "https:" + thumb
	}

	return models.Scene{
		ID:          slug,
		SiteID:      s.cfg.id,
		StudioURL:   studioURL,
		Title:       m.Name,
		URL:         m.URL,
		Thumbnail:   thumb,
		Date:        date,
		Description: m.Description,
		Performers:  performers,
		Studio:      s.cfg.studio,
		ScrapedAt:   now,
	}
}
