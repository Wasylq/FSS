package analacrobats

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const siteID = "analacrobats"

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?analacrobats\.com`)

type Scraper struct {
	client *http.Client
	base   string
}

func New() *Scraper {
	return &Scraper{
		client: httpx.NewClient(30 * time.Second),
		base:   "https://www.analacrobats.com",
	}
}

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string         { return siteID }
func (s *Scraper) Patterns() []string { return []string{"analacrobats.com"} }
func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	cardRe      = regexp.MustCompile(`(?s)<div class="item-col col -video"[^>]*>(.+?)</div>\s*</div>\s*</div>\s*</div>`)
	sceneURLRe  = regexp.MustCompile(`<a href="([^"]*?/video/[^"]+)"`)
	sceneIDRe   = regexp.MustCompile(`-(\d+)\.html`)
	titleRe     = regexp.MustCompile(`<span class="item-name">([^<]+)</span>`)
	thumbRe     = regexp.MustCompile(`data-src="([^"]+)"`)
	dateRe      = regexp.MustCompile(`Date:\s*(\d{2}\.\d{2}\.\d{4})`)
	durationRe  = regexp.MustCompile(`Time:\s*(\d+:\d{2}(?::\d{2})?)`)
	performerRe = regexp.MustCompile(`<a\s+title="([^"]+)"\s+href="[^"]*?/models/`)
	maxPageRe   = regexp.MustCompile(`page(\d+)\.html`)
)

type sceneItem struct {
	id         string
	title      string
	url        string
	thumb      string
	date       time.Time
	duration   int
	performers []string
}

func parseListingPage(body []byte) []sceneItem {
	matches := cardRe.FindAllSubmatch(body, -1)
	items := make([]sceneItem, 0, len(matches))

	for _, m := range matches {
		block := string(m[1])
		var item sceneItem

		if sm := sceneURLRe.FindStringSubmatch(block); sm != nil {
			item.url = sm[1]
			if idm := sceneIDRe.FindStringSubmatch(item.url); idm != nil {
				item.id = idm[1]
			}
		}
		if item.id == "" {
			continue
		}

		if sm := titleRe.FindStringSubmatch(block); sm != nil {
			item.title = strings.TrimSpace(sm[1])
		}

		if sm := thumbRe.FindStringSubmatch(block); sm != nil {
			thumb := sm[1]
			if strings.HasPrefix(thumb, "//") {
				thumb = "https:" + thumb
			}
			item.thumb = thumb
		}

		if sm := dateRe.FindStringSubmatch(block); sm != nil {
			if t, err := time.Parse("02.01.2006", sm[1]); err == nil {
				item.date = t.UTC()
			}
		}

		if sm := durationRe.FindStringSubmatch(block); sm != nil {
			item.duration = parseDuration(sm[1])
		}

		for _, pm := range performerRe.FindAllStringSubmatch(block, -1) {
			name := strings.TrimSpace(pm[1])
			if name != "" {
				item.performers = append(item.performers, name)
			}
		}

		items = append(items, item)
	}
	return items
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

func estimateTotal(body []byte, perPage int) int {
	max := 1
	for _, m := range maxPageRe.FindAllSubmatch(body, -1) {
		n, _ := strconv.Atoi(string(m[1]))
		if n > max {
			max = n
		}
	}
	return max * perPage
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
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

		pageURL := s.base + "/most-recent/"
		if page > 1 {
			pageURL += fmt.Sprintf("page%d.html", page)
		}

		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		scenes := parseListingPage(body)
		if len(scenes) == 0 {
			return
		}

		if page == 1 {
			total := estimateTotal(body, len(scenes))
			if total > 0 {
				select {
				case out <- scraper.Progress(total):
				case <-ctx.Done():
					return
				}
			}
		}

		for _, item := range scenes {
			if opts.KnownIDs[item.id] {
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
				return
			}
			select {
			case out <- scraper.Scene(item.toScene(studioURL, s.base, now)):
			case <-ctx.Done():
				return
			}
		}
	}
}

func (item sceneItem) toScene(studioURL, base string, now time.Time) models.Scene {
	url := item.url
	if strings.HasPrefix(url, "/") {
		url = base + url
	}
	return models.Scene{
		ID:         item.id,
		SiteID:     siteID,
		StudioURL:  studioURL,
		Title:      item.title,
		URL:        url,
		Thumbnail:  item.thumb,
		Date:       item.date,
		Duration:   item.duration,
		Performers: item.performers,
		Studio:     "Anal Acrobats",
		ScrapedAt:  now,
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
	return httpx.ReadBody(resp.Body)
}
