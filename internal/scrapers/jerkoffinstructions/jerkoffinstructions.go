package jerkoffinstructions

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

const siteBase = "https://jerkoffinstructions.com"

type Scraper struct {
	client *http.Client
}

func New() *Scraper {
	c := httpx.NewClient(30 * time.Second)
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &Scraper{client: c}
}

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return "jerkoffinstructions" }

func (s *Scraper) Patterns() []string {
	return []string{"jerkoffinstructions.com"}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?jerkoffinstructions\.com(/|$)`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, opts, out)
	return out, nil
}

// ---- runner ----

func (s *Scraper) run(ctx context.Context, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
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

		body, err := s.fetchPage(ctx, page)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		cards, total := parseListingPage(body)

		if page == 1 && total > 0 {
			select {
			case out <- scraper.Progress(total):
			case <-ctx.Done():
				return
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
			scene := buildScene(c, now)
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

// ---- HTTP ----

// The server returns 302 with Location: /index.php on every request,
// but the response body still contains the full page HTML. The client
// is configured not to follow redirects so we can read the body.
func (s *Scraper) fetchPage(ctx context.Context, page int) ([]byte, error) {
	u := fmt.Sprintf("%s/tour.php?p=%d", siteBase, page)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", httpx.UserAgentFirefox)
	req.Header.Set("Cookie", "free_adult=dark_mode")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound {
		return nil, &httpx.StatusError{StatusCode: resp.StatusCode}
	}

	return httpx.ReadBody(resp.Body)
}

// ---- parsing ----

type listingCard struct {
	id          string
	url         string
	title       string
	description string
	thumbnail   string
	date        time.Time
	performers  []string
	tags        []string
	duration    int
}

var (
	cardBlockRe = regexp.MustCompile(`(?s)<div class="curvy">\s*<span id="ctl">.*?</table>\s*</div>`)
	videoURLRe  = regexp.MustCompile(`<a href="/videos/(\d+)/">`)
	thumbRe     = regexp.MustCompile(`<img src="(/cover_images/[^"]+)" class="photo"`)
	titleRe     = regexp.MustCompile(`<div class="title">([^<]+)</div>`)
	descRe      = regexp.MustCompile(`(?s)<p>(.*?)(?:&nbsp;)?<a href="/videos/\d+/">more</a></p>`)
	dateRe      = regexp.MustCompile(`Date Added:\s*(\d{2}/\d{2}/\d{4})`)
	starringRe  = regexp.MustCompile(`(?s)Starring:\s*(.*?)<br>`)
	perfLinkRe  = regexp.MustCompile(`>([^<]+)</a>`)
	durationRe  = regexp.MustCompile(`Running Time:\s*(\d+)\s+mins`)
	keywordsRe  = regexp.MustCompile(`(?s)<div class="keywords">\s*(.*?)</div>`)
	kwLinkRe    = regexp.MustCompile(`>([^<]+)</a>`)
	totalRe     = regexp.MustCompile(`(\d+)\s+Video\(s\) Found`)
)

func parseListingPage(body []byte) ([]listingCard, int) {
	total := 0
	if m := totalRe.FindSubmatch(body); m != nil {
		total, _ = strconv.Atoi(string(m[1]))
	}

	blocks := cardBlockRe.FindAll(body, -1)
	cards := make([]listingCard, 0, len(blocks))

	for _, block := range blocks {
		c := listingCard{}

		if m := videoURLRe.FindSubmatch(block); m != nil {
			c.id = string(m[1])
			c.url = siteBase + "/videos/" + c.id + "/"
		}
		if c.id == "" {
			continue
		}

		if m := thumbRe.FindSubmatch(block); m != nil {
			c.thumbnail = siteBase + string(m[1])
		}

		if m := titleRe.FindSubmatch(block); m != nil {
			t := string(m[1])
			if !strings.Contains(t, "Video(s) Found") {
				c.title = html.UnescapeString(strings.TrimSpace(t))
			}
		}
		if c.title == "" {
			ms := titleRe.FindAllSubmatch(block, -1)
			for _, tm := range ms {
				t := string(tm[1])
				if !strings.Contains(t, "Video(s) Found") {
					c.title = html.UnescapeString(strings.TrimSpace(t))
					break
				}
			}
		}

		if m := descRe.FindSubmatch(block); m != nil {
			c.description = html.UnescapeString(strings.TrimSpace(string(m[1])))
		}

		if m := dateRe.FindSubmatch(block); m != nil {
			if t, err := time.Parse("01/02/2006", string(m[1])); err == nil {
				c.date = t.UTC()
			}
		}

		if m := starringRe.FindSubmatch(block); m != nil {
			for _, pm := range perfLinkRe.FindAllSubmatch(m[1], -1) {
				name := strings.TrimSpace(html.UnescapeString(string(pm[1])))
				if name != "" {
					c.performers = append(c.performers, name)
				}
			}
		}

		if m := durationRe.FindSubmatch(block); m != nil {
			mins, _ := strconv.Atoi(string(m[1]))
			c.duration = mins * 60
		}

		if m := keywordsRe.FindSubmatch(block); m != nil {
			for _, km := range kwLinkRe.FindAllSubmatch(m[1], -1) {
				tag := strings.TrimSpace(html.UnescapeString(string(km[1])))
				if tag != "" {
					c.tags = append(c.tags, tag)
				}
			}
		}

		cards = append(cards, c)
	}

	return cards, total
}

func buildScene(c listingCard, now time.Time) models.Scene {
	return models.Scene{
		ID:          c.id,
		SiteID:      "jerkoffinstructions",
		StudioURL:   siteBase,
		Title:       c.title,
		Description: c.description,
		URL:         c.url,
		Thumbnail:   c.thumbnail,
		Date:        c.date,
		Duration:    c.duration,
		Performers:  c.performers,
		Tags:        c.tags,
		Studio:      "Jerk Off Instructions",
		ScrapedAt:   now,
	}
}
