package wankitnowvr

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

const siteBase = "https://wankitnowvr.com"

type Scraper struct {
	client *http.Client
}

func New() *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second)}
}

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return "wankitnowvr" }
func (s *Scraper) Patterns() []string {
	return []string{
		"wankitnowvr.com",
		"wankitnowvr.com/models/{slug}/{id}",
	}
}

var (
	matchRe = regexp.MustCompile(`^https?://(?:www\.)?wankitnowvr\.com`)
	modelRe = regexp.MustCompile(`/models/`)
)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()

	if modelRe.MatchString(studioURL) {
		s.scrapeModelPage(ctx, studioURL, opts, out, now)
	} else {
		s.scrapeListing(ctx, studioURL, opts, out, now)
	}
}

func (s *Scraper) scrapeModelPage(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult, now time.Time) {
	body, err := s.fetchURL(ctx, studioURL)
	if err != nil {
		select {
		case out <- scraper.Error(err):
		case <-ctx.Done():
		}
		return
	}
	cards := parseListingPage(body)
	if len(cards) > 0 {
		select {
		case out <- scraper.Progress(len(cards)):
		case <-ctx.Done():
			return
		}
	}
	for _, c := range cards {
		if opts.KnownIDs[c.id] {
			continue
		}
		select {
		case out <- scraper.Scene(buildScene(c, studioURL, now)):
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scraper) scrapeListing(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult, now time.Time) {
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

		body, err := s.fetchPage(ctx, page)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		cards := parseListingPage(body)

		if page == 1 {
			if total := parseLastPage(body); total > 0 {
				select {
				case out <- scraper.Progress(total * len(cards)):
				case <-ctx.Done():
					return
				}
			}
		}

		if len(cards) == 0 {
			return
		}

		stoppedEarly := false
		for _, c := range cards {
			if opts.KnownIDs[c.id] {
				stoppedEarly = true
				break
			}
			select {
			case out <- scraper.Scene(buildScene(c, studioURL, now)):
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

func (s *Scraper) fetchPage(ctx context.Context, page int) ([]byte, error) {
	return s.fetchURL(ctx, fmt.Sprintf("%s/videos?page=%d", siteBase, page))
}

func (s *Scraper) fetchURL(ctx context.Context, u string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: u,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentChrome,
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}

type listingCard struct {
	id         string
	slug       string
	title      string
	thumbnail  string
	date       time.Time
	duration   int
	performers []string
}

var (
	cardRe      = regexp.MustCompile(`(?s)<div class="card border-0 px-0">(.*?)</div>\s*</div>\s*</div>`)
	videoURLRe  = regexp.MustCompile(`href="https://wankitnowvr\.com/videos/([^"]+)/(\d+)"`)
	thumbRe     = regexp.MustCompile(`<img class="card-img-top" src="([^"]+)"`)
	titleRe     = regexp.MustCompile(`<a href="https://wankitnowvr\.com/videos/[^"]+" class="card-link">([^<]+)</a>`)
	subtitleRe  = regexp.MustCompile(`(?i)([A-Za-z]+ \d{1,2}, \d{4})\s*\|\s*Duration:\s*(\d+:\d+)`)
	performerRe = regexp.MustCompile(`href="https://wankitnowvr\.com/models/[^"]+" title="[^"]*" class="btn-link">([^<]+)</a>`)
	lastPageRe  = regexp.MustCompile(`page=(\d+)"[^>]*>\d+</a>\s*</li>\s*<li[^>]*>\s*<a[^>]*>›</a>`)
)

func parseListingPage(body []byte) []listingCard {
	blocks := cardRe.FindAll(body, -1)
	cards := make([]listingCard, 0, len(blocks))

	for _, block := range blocks {
		c := listingCard{}

		if m := videoURLRe.FindSubmatch(block); m != nil {
			c.slug = string(m[1])
			c.id = string(m[2])
		}
		if c.id == "" {
			continue
		}

		if m := thumbRe.FindSubmatch(block); m != nil {
			c.thumbnail = string(m[1])
		}

		if m := titleRe.FindSubmatch(block); m != nil {
			c.title = html.UnescapeString(strings.TrimSpace(string(m[1])))
		}

		if m := subtitleRe.FindSubmatch(block); m != nil {
			if t, err := time.Parse("January 2, 2006", string(m[1])); err == nil {
				c.date = t.UTC()
			}
			c.duration = parseDuration(string(m[2]))
		}

		for _, pm := range performerRe.FindAllSubmatch(block, -1) {
			name := strings.TrimSpace(html.UnescapeString(string(pm[1])))
			if name != "" {
				c.performers = append(c.performers, name)
			}
		}

		cards = append(cards, c)
	}

	return cards
}

func parseLastPage(body []byte) int {
	m := lastPageRe.FindSubmatch(body)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(string(m[1]))
	return n
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

func buildScene(c listingCard, studioURL string, now time.Time) models.Scene {
	return models.Scene{
		ID:         c.id,
		SiteID:     "wankitnowvr",
		StudioURL:  studioURL,
		Title:      c.title,
		URL:        fmt.Sprintf("%s/videos/%s/%s", siteBase, c.slug, c.id),
		Thumbnail:  c.thumbnail,
		Date:       c.date,
		Duration:   c.duration,
		Performers: c.performers,
		Studio:     "Wank It Now VR",
		ScrapedAt:  now,
	}
}
