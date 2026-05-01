package sexmexutil

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"regexp"
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
	Cfg    SiteConfig
	Client *http.Client
}

func New(cfg SiteConfig) *Scraper {
	return &Scraper{
		Cfg:    cfg,
		Client: httpx.NewClient(30 * time.Second),
	}
}

func (s *Scraper) ID() string               { return s.Cfg.ID }
func (s *Scraper) Patterns() []string       { return s.Cfg.Patterns }
func (s *Scraper) MatchesURL(u string) bool { return s.Cfg.MatchRe.MatchString(u) }

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
	parts := strings.SplitN(slug, "/", 2)
	return fmt.Sprintf("%s/tour/%s/%s_%d_d.html", siteBase, parts[0], parts[1], page)
}

// ---- runner ----

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	slug := resolveListingSlug(studioURL, s.Cfg.SiteBase)

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

		u := pageURL(s.Cfg.SiteBase, slug, page)
		body, err := s.fetchPage(ctx, u)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		cards := parseCards(body)
		if len(cards) == 0 {
			return
		}

		now := time.Now().UTC()
		stoppedEarly := false
		for _, c := range cards {
			scene := s.cardToScene(studioURL, c, now)
			if len(opts.KnownIDs) > 0 && opts.KnownIDs[scene.ID] {
				stoppedEarly = true
				break
			}
			select {
			case out <- scraper.Scene(scene):
			case <-ctx.Done():
				return
			}
		}

		if stoppedEarly {
			select {
			case out <- scraper.StoppedEarly():
			case <-ctx.Done():
			}
			return
		}
	}
}

// ---- page fetch ----

// fetchPage fetches HTML, accepting HTTP 500 responses because
// SexMex's CMS returns 500 status with valid HTML on some pages.
func (s *Scraper) fetchPage(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", httpx.UserAgentFirefox)
	req.Header.Set("Accept", "text/html")

	resp, err := s.Client.Do(req)
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

var cardRe = regexp.MustCompile(`(?s)<div[^>]*data-setid="(\d+)"[^>]*>.*?</a></div>`)

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
		SiteID:      s.Cfg.ID,
		StudioURL:   studioURL,
		Title:       title,
		URL:         c.url,
		Thumbnail:   c.thumbnail,
		Description: c.description,
		Performers:  c.performers,
		Date:        c.date,
		Studio:      s.Cfg.Studio,
		ScrapedAt:   now,
	}
}
