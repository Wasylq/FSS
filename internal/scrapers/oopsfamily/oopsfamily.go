package oopsfamily

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const siteBase = "https://oopsfamily.com"

type Scraper struct {
	client *http.Client
}

func New() *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second)}
}

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return "oopsfamily" }

func (s *Scraper) Patterns() []string {
	return []string{
		"oopsfamily.com",
		"oopsfamily.com/model/{slug}",
		"oopsfamily.com/tag/{slug}",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?oopsfamily\.com(/|$)`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// ---- URL routing ----

var (
	modelPathRe = regexp.MustCompile(`/model/([\w-]+)`)
	tagPathRe   = regexp.MustCompile(`/tag/([\w-]+)`)
)

func resolveListingBase(studioURL string) string {
	if m := modelPathRe.FindStringSubmatch(studioURL); m != nil {
		return siteBase + "/model/" + m[1]
	}
	if m := tagPathRe.FindStringSubmatch(studioURL); m != nil {
		return siteBase + "/tag/" + m[1]
	}
	return siteBase + "/video"
}

// ---- runner ----

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	base := resolveListingBase(studioURL)

	var collected []listingCard
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

		cards, hasNext, err := s.fetchListingPage(ctx, base, page)
		if err != nil {
			select {
			case out <- scraper.SceneResult{Err: fmt.Errorf("page %d: %w", page, err)}:
			case <-ctx.Done():
			}
			return
		}
		if len(cards) == 0 {
			break
		}

		for _, c := range cards {
			if len(opts.KnownIDs) > 0 && opts.KnownIDs[c.id] {
				stoppedEarly = true
				break
			}
			collected = append(collected, c)
		}

		if stoppedEarly || !hasNext {
			break
		}
	}

	if len(collected) == 0 {
		if stoppedEarly {
			select {
			case out <- scraper.SceneResult{StoppedEarly: true}:
			case <-ctx.Done():
			}
		}
		return
	}

	details := s.fetchDetails(ctx, collected, opts.Delay)

	now := time.Now().UTC()
	for _, c := range collected {
		scene := buildScene(studioURL, c, details[c.url], now)
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

// ---- listing parsing ----

type listingCard struct {
	id         string
	url        string
	title      string
	thumbnail  string
	duration   int
	performers []string
}

var (
	cardBlockRe = regexp.MustCompile(`(?s)<div class="video-card__item"[^>]*>(.*?)<div class="video-card__icons">`)
	cardURLRe   = regexp.MustCompile(`href="(https://oopsfamily\.com/video/[^"]+)" class="video-card__title"`)
	cardTitleRe = regexp.MustCompile(`(?s)class="video-card__title">\s*(.+?)\s*</a>`)
	cardThumbRe = regexp.MustCompile(`class="image-container"[^>]*>\s*<img src="([^"]+)"`)
	cardDurRe   = regexp.MustCompile(`video-card__quality">\s*(?:<img[^>]*>)?\s*(\d+):(\d+)`)
	cardPerfsRe = regexp.MustCompile(`(?s)video-card__actors[^>]*>(.*?)</div>`)
	cardPerfRe  = regexp.MustCompile(`(?s)<a[^>]*>\s*(.+?)\s*</a>`)
	nextActiveRe = regexp.MustCompile(`pagination__next icon-right-arr"`)
)

var sceneIDRe = regexp.MustCompile(`/video/[\w-]+-(\w+)$`)

func extractID(rawURL string) string {
	if m := sceneIDRe.FindStringSubmatch(rawURL); m != nil {
		return m[1]
	}
	return ""
}

func parseListingPage(body []byte) ([]listingCard, bool) {
	blocks := cardBlockRe.FindAllSubmatch(body, -1)
	cards := make([]listingCard, 0, len(blocks))

	for _, bm := range blocks {
		block := bm[1]
		c := listingCard{}

		if m := cardURLRe.FindSubmatch(block); m != nil {
			c.url = string(m[1])
			c.id = extractID(c.url)
		}
		if c.id == "" {
			continue
		}

		if m := cardTitleRe.FindSubmatch(block); m != nil {
			c.title = strings.TrimSpace(string(m[1]))
		}
		if m := cardThumbRe.FindSubmatch(block); m != nil {
			c.thumbnail = string(m[1])
		}
		if m := cardDurRe.FindSubmatch(block); m != nil {
			mins := atoi(string(m[1]))
			secs := atoi(string(m[2]))
			c.duration = mins*60 + secs
		}
		if m := cardPerfsRe.FindSubmatch(block); m != nil {
			for _, pm := range cardPerfRe.FindAllSubmatch(m[1], -1) {
				name := strings.TrimSpace(string(pm[1]))
				if name != "" {
					c.performers = append(c.performers, name)
				}
			}
		}

		cards = append(cards, c)
	}

	hasNext := nextActiveRe.Match(body)
	return cards, hasNext
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

func (s *Scraper) fetchListingPage(ctx context.Context, base string, page int) ([]listingCard, bool, error) {
	u := fmt.Sprintf("%s?page=%d", base, page)
	body, err := s.fetchHTML(ctx, u)
	if err != nil {
		return nil, false, err
	}
	cards, hasNext := parseListingPage(body)
	return cards, hasNext, nil
}

// ---- detail page parsing ----

type detailData struct {
	date time.Time
	tags []string
}

type jsonLD struct {
	UploadDate string      `json:"uploadDate"`
	Genre      interface{} `json:"genre"`
}

var jsonLDRe = regexp.MustCompile(`(?s)<script type="application/ld\+json">\s*(\{.*?})\s*</script>`)

func parseDetailPage(body []byte) detailData {
	d := detailData{}
	m := jsonLDRe.FindSubmatch(body)
	if m == nil {
		return d
	}

	var ld jsonLD
	if err := json.Unmarshal(m[1], &ld); err != nil {
		return d
	}

	if ld.UploadDate != "" {
		if t, err := time.Parse(time.RFC3339, ld.UploadDate); err == nil {
			d.date = t.UTC()
		}
	}

	switch v := ld.Genre.(type) {
	case []interface{}:
		for _, g := range v {
			if s, ok := g.(string); ok && s != "Pornography" {
				d.tags = append(d.tags, s)
			}
		}
	}

	return d
}

func (s *Scraper) fetchDetails(ctx context.Context, cards []listingCard, delay time.Duration) map[string]detailData {
	results := make(map[string]detailData, len(cards))
	var mu sync.Mutex

	const workers = 4
	work := make(chan listingCard, len(cards))
	for _, c := range cards {
		work <- c
	}
	close(work)

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range work {
				if ctx.Err() != nil {
					return
				}
				if delay > 0 {
					select {
					case <-time.After(delay):
					case <-ctx.Done():
						return
					}
				}
				body, err := s.fetchHTML(ctx, c.url)
				if err != nil {
					continue
				}
				d := parseDetailPage(body)
				mu.Lock()
				results[c.url] = d
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	return results
}

// ---- scene builder ----

func buildScene(studioURL string, c listingCard, d detailData, now time.Time) models.Scene {
	date := d.date
	// Unreleased scenes return 403 on the detail page, so no date is available.
	// Use scrape time as fallback since the scene is already listed.
	if date.IsZero() {
		date = now
	}
	return models.Scene{
		ID:         c.id,
		SiteID:     "oopsfamily",
		StudioURL:  studioURL,
		Title:      c.title,
		URL:        c.url,
		Thumbnail:  c.thumbnail,
		Duration:   c.duration,
		Performers: c.performers,
		Date:       date,
		Tags:       d.tags,
		Studio:     "OopsFamily",
		Width:      3840,
		Height:     2160,
		Resolution: "2160p",
		ScrapedAt:  now,
	}
}

// ---- HTTP ----

func (s *Scraper) fetchHTML(ctx context.Context, rawURL string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: rawURL,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentFirefox,
			"Accept":     "text/html",
			"Cookie":     "adult=1",
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return io.ReadAll(resp.Body)
}
