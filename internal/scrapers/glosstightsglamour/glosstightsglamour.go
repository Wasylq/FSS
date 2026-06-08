// Package glosstightsglamour scrapes glosstightsglamour.com, a Glamose network
// site running on ElevatedX CMS. HTML listing with page_N.html pagination.
package glosstightsglamour

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
	"github.com/Wasylq/FSS/parseutil"
	"github.com/Wasylq/FSS/scraper"
)

const siteBase = "https://www.glosstightsglamour.com"

type Scraper struct {
	client *http.Client
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func New() *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second)}
}

func init() { scraper.Register(New()) }

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?glosstightsglamour\.com\b`)

func (s *Scraper) ID() string               { return "glosstightsglamour" }
func (s *Scraper) Patterns() []string       { return []string{"glosstightsglamour.com/"} }
func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	cardRe   = regexp.MustCompile(`(?s)<a\s+href="(/updates/[^"]+\.html)"[^>]*>.*?<img[^>]+src="([^"]+)"[^>]*alt="([^"]*)"`)
	modelRe  = regexp.MustCompile(`/models/[^"]*">([^<]+)</a>`)
	dateRe   = regexp.MustCompile(`(\d{2}/\d{2}/\d{4})`)
	lastPgRe = regexp.MustCompile(`/updates/page_(\d+)\.html`)
)

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	scraper.Paginate(ctx, opts, "glosstightsglamour", out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		var u string
		if page == 1 {
			u = siteBase + "/updates/"
		} else {
			u = fmt.Sprintf("%s/updates/page_%d.html", siteBase, page)
		}

		body, err := s.fetchHTML(ctx, u)
		if err != nil {
			return scraper.PageResult{}, err
		}

		scenes := parseListingPage(body, studioURL)

		total := 0
		done := false
		if page == 1 {
			maxPg := parseMaxPage(body)
			if maxPg > 0 {
				total = maxPg * 12
			}
		}
		if len(scenes) == 0 {
			done = true
		}

		return scraper.PageResult{Scenes: scenes, Total: total, Done: done}, nil
	})
}

func parseListingPage(body []byte, studioURL string) []models.Scene {
	now := time.Now().UTC()
	text := string(body)

	cards := cardRe.FindAllStringSubmatchIndex(text, -1)
	var scenes []models.Scene
	seen := make(map[string]bool)

	for i, loc := range cards {
		path := text[loc[2]:loc[3]]
		thumb := text[loc[4]:loc[5]]
		title := html.UnescapeString(text[loc[6]:loc[7]])

		if title == "" || seen[path] {
			continue
		}
		seen[path] = true

		slug := extractSlug(path)
		if slug == "" {
			continue
		}

		scene := models.Scene{
			ID:        slug,
			SiteID:    "glosstightsglamour",
			StudioURL: studioURL,
			Title:     title,
			URL:       siteBase + path,
			Studio:    "Gloss Tights Glamour",
			ScrapedAt: now,
		}

		if !strings.HasPrefix(thumb, "http") {
			thumb = siteBase + "/" + strings.TrimPrefix(thumb, "/")
		}
		scene.Thumbnail = thumb

		end := len(text)
		if i+1 < len(cards) {
			end = cards[i+1][0]
		}
		block := text[loc[0]:end]

		if m := modelRe.FindStringSubmatch(block); m != nil {
			scene.Performers = []string{strings.TrimSpace(m[1])}
		}

		if m := dateRe.FindStringSubmatch(block); m != nil {
			if t, err := time.Parse("02/01/2006", m[1]); err == nil {
				scene.Date = t.UTC()
			}
		}

		scenes = append(scenes, scene)
	}
	return scenes
}

var slugRe = regexp.MustCompile(`/updates/([^/]+)\.html`)

func extractSlug(path string) string {
	if m := slugRe.FindStringSubmatch(path); m != nil {
		return m[1]
	}
	return ""
}

func parseMaxPage(body []byte) int {
	max := 0
	for _, m := range lastPgRe.FindAllSubmatch(body, -1) {
		n := parseutil.ParseDurationColon("0:" + string(m[1]))
		if n == 0 {
			continue
		}
		if int(m[1][0]-'0') > 0 {
			val := 0
			for _, c := range m[1] {
				val = val*10 + int(c-'0')
			}
			if val > max {
				max = val
			}
		}
	}
	return max
}

func (s *Scraper) fetchHTML(ctx context.Context, rawURL string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: rawURL,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentFirefox,
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}
