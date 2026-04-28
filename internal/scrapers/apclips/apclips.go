package apclips

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const (
	defaultBase = "https://apclips.com"
	perPage     = 60
)

type Scraper struct {
	client *http.Client
	base   string
}

func New() *Scraper {
	return &Scraper{
		client: httpx.NewClient(30 * time.Second),
		base:   defaultBase,
	}
}

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return "apclips" }

func (s *Scraper) Patterns() []string {
	return []string{
		"apclips.com/{creator_slug}",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?apclips\.com/([a-zA-Z0-9_-]+)(?:/videos)?/?(?:\?.*)?$`)

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	slug := slugFromURL(studioURL)
	if slug == "" {
		return nil, fmt.Errorf("apclips: cannot extract creator slug from %q", studioURL)
	}
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, slug, opts, out)
	return out, nil
}

func slugFromURL(u string) string {
	if m := matchRe.FindStringSubmatch(u); m != nil {
		return m[1]
	}
	parts := strings.Split(strings.TrimRight(u, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		seg := parts[i]
		if seg != "" && seg != "videos" {
			return seg
		}
	}
	return ""
}

type card struct {
	id          string
	title       string
	description string
	price       float64
	duration    int
	thumbnail   string
	preview     string
	detailPath  string
	creatorName string
	date        time.Time
	tags        []string
}

func (s *Scraper) run(ctx context.Context, studioURL, slug string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
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

		pageURL := fmt.Sprintf("%s/%s/videos?sort=date-new&page=%d", s.base, slug, page)
		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			send(ctx, out, scraper.Error(fmt.Errorf("page %d: %w", page, err)))
			return
		}

		cards, total := parseListingPage(body)
		if len(cards) == 0 {
			return
		}

		if page == 1 && total > 0 {
			send(ctx, out, scraper.Progress(total))
		}

		for _, c := range cards {
			if opts.KnownIDs != nil && opts.KnownIDs[c.id] {
				send(ctx, out, scraper.StoppedEarly())
				return
			}

			if c.detailPath != "" {
				if opts.Delay > 0 {
					select {
					case <-time.After(opts.Delay):
					case <-ctx.Done():
						return
					}
				}
				detailBody, err := s.fetchPage(ctx, s.base+c.detailPath)
				if err == nil {
					c.date, c.tags = parseDetailPage(detailBody)
				}
			}

			scene := toScene(c, slug, studioURL, now)
			if !send(ctx, out, scraper.Scene(scene)) {
				return
			}
		}

		if total > 0 && page*perPage >= total {
			break
		}
		if len(cards) < perPage {
			break
		}
	}
}

func (s *Scraper) fetchPage(ctx context.Context, url string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: url,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentChrome,
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return io.ReadAll(resp.Body)
}

var (
	cardBlockRe    = regexp.MustCompile(`(?s)class="card thumb-block[^"]*"`)
	contentCodeRe  = regexp.MustCompile(`data-content-code="([^"]+)"`)
	contentPriceRe = regexp.MustCompile(`data-content-price="([^"]+)"`)
	hrefRe         = regexp.MustCompile(`href="(/[^"]+)"`)
	dataSrcRe      = regexp.MustCompile(`data-src="([^"]+)"`)
	dataPreviewRe  = regexp.MustCompile(`data-preview="([^"]+)"`)
	durationRe     = regexp.MustCompile(`class="item-details">\s*([^<]+)`)
	titleRe        = regexp.MustCompile(`class="item-title[^"]*">\s*([^<]+)`)
	descRe         = regexp.MustCompile(`class="item-desc[^"]*">\s*([^<]+)`)
	altRe          = regexp.MustCompile(`alt="[^"]*video from ([^"]+)"`)
	totalRe        = regexp.MustCompile(`of (\d+) Results`)
)

func parseListingPage(body []byte) ([]card, int) {
	total := 0
	if m := totalRe.FindSubmatch(body); m != nil {
		total, _ = strconv.Atoi(string(m[1]))
	}

	locs := cardBlockRe.FindAllIndex(body, -1)
	cards := make([]card, 0, len(locs))

	for i, loc := range locs {
		end := len(body)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		block := body[loc[0]:end]

		var c card

		if m := contentCodeRe.FindSubmatch(block); m != nil {
			c.id = string(m[1])
		}
		if c.id == "" {
			continue
		}

		if m := titleRe.FindSubmatch(block); m != nil {
			c.title = html.UnescapeString(strings.TrimSpace(string(m[1])))
		}
		if m := descRe.FindSubmatch(block); m != nil {
			c.description = html.UnescapeString(strings.TrimSpace(string(m[1])))
		}
		if m := contentPriceRe.FindSubmatch(block); m != nil {
			c.price, _ = strconv.ParseFloat(string(m[1]), 64)
		}
		if m := durationRe.FindSubmatch(block); m != nil {
			c.duration = parseDuration(string(m[1]))
		}
		if m := dataSrcRe.FindSubmatch(block); m != nil {
			c.thumbnail = resolveURL(defaultBase, strings.TrimSpace(string(m[1])))
		}
		if m := dataPreviewRe.FindSubmatch(block); m != nil {
			c.preview = strings.TrimSpace(string(m[1]))
		}
		if m := hrefRe.FindSubmatch(block); m != nil {
			c.detailPath = string(m[1])
		}
		if m := altRe.FindSubmatch(block); m != nil {
			c.creatorName = html.UnescapeString(strings.TrimSpace(string(m[1])))
		}

		cards = append(cards, c)
	}

	return cards, total
}

var (
	dateRe     = regexp.MustCompile(`<time datetime="([^"]+)"`)
	tagCloudRe = regexp.MustCompile(`(?s)class="tag-cloud">(.*?)</div>`)
	tagLinkRe  = regexp.MustCompile(`class="tag-link"[^>]*>([^<]+)`)
)

func parseDetailPage(body []byte) (time.Time, []string) {
	var date time.Time
	if m := dateRe.FindSubmatch(body); m != nil {
		date = parseDate(string(m[1]))
	}

	var tags []string
	if m := tagCloudRe.FindSubmatch(body); m != nil {
		for _, tm := range tagLinkRe.FindAllSubmatch(m[1], -1) {
			tag := strings.TrimSpace(string(tm[1]))
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}

	return date, tags
}

var ordinalRe = regexp.MustCompile(`(\d+)(st|nd|rd|th)`)

func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	s = ordinalRe.ReplaceAllString(s, "$1")
	t, err := time.Parse("Jan 2, 2006", s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

func parseDuration(s string) int {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ":")
	switch len(parts) {
	case 2:
		m, _ := strconv.Atoi(parts[0])
		sec, _ := strconv.Atoi(parts[1])
		return m*60 + sec
	case 3:
		h, _ := strconv.Atoi(parts[0])
		m, _ := strconv.Atoi(parts[1])
		sec, _ := strconv.Atoi(parts[2])
		return h*3600 + m*60 + sec
	default:
		return 0
	}
}

func resolveURL(base, u string) string {
	if u == "" || strings.HasPrefix(u, "http") {
		return u
	}
	return base + u
}

func toScene(c card, slug, studioURL string, now time.Time) models.Scene {
	sc := models.Scene{
		ID:          c.id,
		SiteID:      "apclips",
		StudioURL:   studioURL,
		Title:       c.title,
		URL:         defaultBase + c.detailPath,
		Duration:    c.duration,
		Thumbnail:   c.thumbnail,
		Preview:     c.preview,
		Description: c.description,
		Studio:      c.creatorName,
		Tags:        c.tags,
		Date:        c.date,
		ScrapedAt:   now,
	}

	if sc.Studio == "" {
		sc.Studio = slug
	}

	if c.creatorName != "" {
		sc.Performers = []string{c.creatorName}
	}

	sc.AddPrice(models.PriceSnapshot{
		Date:    now,
		Regular: c.price,
		IsFree:  c.price == 0,
	})

	return sc
}

func send(ctx context.Context, ch chan<- scraper.SceneResult, r scraper.SceneResult) bool {
	select {
	case ch <- r:
		return true
	case <-ctx.Done():
		return false
	}
}
