package sexmexutil

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

type SiteConfig struct {
	ID       string
	Studio   string
	SiteBase string
	Patterns []string
	MatchRe  *regexp.Regexp
}

type Scraper struct {
	cfg    SiteConfig
	Client *http.Client
}

func New(cfg SiteConfig) *Scraper {
	return &Scraper{
		cfg:    cfg,
		Client: httpx.NewClient(30 * time.Second),
	}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func (s *Scraper) ID() string               { return s.cfg.ID }
func (s *Scraper) Patterns() []string       { return s.cfg.Patterns }
func (s *Scraper) MatchesURL(u string) bool { return s.cfg.MatchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// ---- URL routing ----

var (
	modelRe    = regexp.MustCompile(`/tour/models/([\w-]+)\.html`)
	categoryRe = regexp.MustCompile(`/tour/categories/([\w-]+)\.html`)
)

func resolveListingSlug(studioURL, siteBase string) string {
	if m := modelRe.FindStringSubmatch(studioURL); m != nil {
		return "models/" + m[1]
	}
	if m := categoryRe.FindStringSubmatch(studioURL); m != nil {
		return "categories/" + m[1]
	}
	return "categories/movies"
}

func pageURL(siteBase, slug string, page int) string {
	if page == 1 {
		return fmt.Sprintf("%s/tour/%s.html", siteBase, slug)
	}
	dir, base, ok := strings.Cut(slug, "/")
	if !ok {
		return fmt.Sprintf("%s/tour/%s_%d_d.html", siteBase, slug, page)
	}
	return fmt.Sprintf("%s/tour/%s/%s_%d_d.html", siteBase, dir, base, page)
}

// ---- runner ----

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	slug := resolveListingSlug(studioURL, s.cfg.SiteBase)
	scraper.Debugf(1, "%s: listing slug: %s", s.cfg.ID, slug)

	now := time.Now().UTC()
	var firstPageCount int
	scraper.Paginate(ctx, opts, s.cfg.ID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		u := pageURL(s.cfg.SiteBase, slug, page)
		body, err := s.fetchPage(ctx, u)
		if err != nil {
			return scraper.PageResult{}, err
		}

		cards := parseCards(body)
		if len(cards) == 0 {
			return scraper.PageResult{}, nil
		}

		total := 0
		if page == 1 {
			firstPageCount = len(cards)
			maxPage := extractMaxPage(body)
			if maxPage > 0 {
				total = maxPage * firstPageCount
			}
		}

		scenes := make([]models.Scene, len(cards))
		for i, c := range cards {
			scenes[i] = s.cardToScene(studioURL, c, now)
		}
		return scraper.PageResult{Scenes: scenes, Total: total}, nil
	})
}

// ---- page fetch ----

// fetchPage fetches HTML, accepting HTTP 500 responses because
// SexMex's CMS returns 500 status with valid HTML on some pages.
// Routes through `httpx.DoWithStatus` so the request still benefits
// from shared transport, retries on network errors, and level-2 debug
// logging — only the status-code classification is delegated here.
func (s *Scraper) fetchPage(ctx context.Context, rawURL string) ([]byte, error) {
	resp, err := httpx.DoWithStatus(ctx, s.Client, httpx.Request{
		URL: rawURL,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentFirefox,
			"Accept":     "text/html",
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusInternalServerError {
		return nil, &httpx.StatusError{StatusCode: resp.StatusCode}
	}
	return httpx.ReadBody(resp.Body)
}

// ---- parsing ----

var (
	cardRe    = regexp.MustCompile(`(?s)<div[^>]*data-setid="(\d+)"[^>]*>.*?</a></div>`)
	pageNumRe = regexp.MustCompile(`_(\d+)_d\.html`)
)

func extractMaxPage(body []byte) int {
	matches := pageNumRe.FindAllSubmatch(body, -1)
	max := 0
	for _, m := range matches {
		n, _ := strconv.Atoi(string(m[1]))
		if n > max {
			max = n
		}
	}
	return max
}

type card struct {
	id          string
	title       string
	url         string
	thumbnail   string
	description string
	performers  []string
	date        time.Time
}

var (
	titleLinkRe = regexp.MustCompile(`(?s)<h5[^>]*class="scene-title[^"]*"[^>]*>.*?<a[^>]*title="([^"]*)"[^>]*href="([^"]*)"`)
	thumbRe     = regexp.MustCompile(`<img[^>]*src="(https://[^"]*sexmex-cdn\.com/tour/content/[^"]+)"`)
	descRe      = regexp.MustCompile(`(?s)<p class="scene-descr[^"]*"[^>]*>(.*?)</p>`)
	performerRe = regexp.MustCompile(`<a[^>]*href="[^"]*/tour/models/[^"]*"[^>]*>([^<]+)</a>`)
	dateRe      = regexp.MustCompile(`(?s)<p class="scene-date[^"]*"[^>]*>\s*(\d{2}/\d{2}/\d{4})\s*</p>`)
)

func parseCards(body []byte) []card {
	matches := cardRe.FindAllSubmatch(body, -1)
	cards := make([]card, 0, len(matches))
	for _, m := range matches {
		c := parseCard(m[0], string(m[1]))
		cards = append(cards, c)
	}
	return cards
}

func parseCard(block []byte, id string) card {
	c := card{id: id}

	if m := titleLinkRe.FindSubmatch(block); m != nil {
		c.title = strings.TrimSpace(html.UnescapeString(string(m[1])))
		c.url = strings.TrimSpace(string(m[2]))
	}

	if m := thumbRe.FindSubmatch(block); m != nil {
		c.thumbnail = string(m[1])
	}

	if m := descRe.FindSubmatch(block); m != nil {
		raw := strings.TrimSpace(string(m[1]))
		raw = strings.TrimSuffix(raw, "\n...")
		raw = strings.TrimSuffix(raw, "...")
		c.description = html.UnescapeString(strings.TrimSpace(raw))
	}

	seen := make(map[string]bool)
	for _, m := range performerRe.FindAllSubmatch(block, -1) {
		name := strings.TrimSpace(string(m[1]))
		if name != "" && !seen[name] {
			seen[name] = true
			c.performers = append(c.performers, name)
		}
	}

	if m := dateRe.FindSubmatch(block); m != nil {
		if t, err := time.Parse("01/02/2006", string(m[1])); err == nil {
			c.date = t.UTC()
		}
	}

	return c
}

func (s *Scraper) cardToScene(studioURL string, c card, now time.Time) models.Scene {
	// Strip performer suffix from title: "TITLE . Performer Name"
	title := c.title
	if idx := strings.LastIndex(title, " . "); idx > 0 {
		title = strings.TrimSpace(title[:idx])
	}

	return models.Scene{
		ID:          c.id,
		SiteID:      s.cfg.ID,
		StudioURL:   studioURL,
		Title:       title,
		URL:         c.url,
		Thumbnail:   c.thumbnail,
		Description: c.description,
		Performers:  c.performers,
		Date:        c.date,
		Studio:      s.cfg.Studio,
		ScrapedAt:   now,
	}
}
