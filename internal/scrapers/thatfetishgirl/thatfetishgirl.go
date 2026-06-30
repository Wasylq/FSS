// Package thatfetishgirl scrapes That Fetish Girl (thatfetishgirl.com), an
// Elevated X tour. The updates/page_{N}.html listing carries every field on
// the card: the scene URL and title, the publish date, the featured model(s),
// the runtime and a content thumbnail. The numeric content id is taken from
// the thumbnail path and used as the stable scene id. There is no detail page
// worth fetching, so the runner parses the listing directly and stops when a
// page yields no new cards.
package thatfetishgirl

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
	"github.com/Wasylq/FSS/parseutil"
	"github.com/Wasylq/FSS/scraper"
)

const (
	siteID     = "thatfetishgirl"
	studioName = "That Fetish Girl"
)

var siteBase = "https://thatfetishgirl.com"

type Scraper struct {
	Client *http.Client
}

func New() *Scraper {
	return &Scraper{Client: httpx.NewClient(30 * time.Second)}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		"thatfetishgirl.com",
		"thatfetishgirl.com/updates/page_{n}.html",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?thatfetishgirl\.com`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	cardSplitRe = regexp.MustCompile(`<div class="latestUpdateB" data-setid="(\d+)"`)
	titleRe     = regexp.MustCompile(`(?s)<h4 class="link_bright">\s*<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	thumbRe     = regexp.MustCompile(`poster_1x="([^"]+)"`)
	contentIDRe = regexp.MustCompile(`contentthumbs/\d+/\d+/(\d+)`)
	dateRe      = regexp.MustCompile(`(?s)<!-- Date -->\s*(\d{2}/\d{2}/\d{4})`)
	durationRe  = regexp.MustCompile(`(\d+)\s*min`)
	modelLinkRe = regexp.MustCompile(`href="[^"]*/models/[^"]*">([^<]+)</a>`)
)

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	seen := make(map[string]bool)
	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := fmt.Sprintf("%s/updates/page_%d.html", siteBase, page)
		body, err := s.get(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}
		scenes := parseListing(body, studioURL, now)
		fresh := scenes[:0]
		for _, sc := range scenes {
			if !seen[sc.ID] {
				seen[sc.ID] = true
				fresh = append(fresh, sc)
			}
		}
		// Pages past the end repeat or empty out — stop when nothing is new.
		if len(fresh) == 0 {
			return scraper.PageResult{Done: true}, nil
		}
		return scraper.PageResult{Scenes: fresh}, nil
	})
}

func parseListing(body []byte, studioURL string, now time.Time) []models.Scene {
	page := string(body)
	locs := cardSplitRe.FindAllStringSubmatchIndex(page, -1)
	scenes := make([]models.Scene, 0, len(locs))
	for i, loc := range locs {
		setID := page[loc[2]:loc[3]]
		start := loc[0]
		end := len(page)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		block := page[start:end]

		scene := models.Scene{
			ID:        setID,
			SiteID:    siteID,
			StudioURL: studioURL,
			Studio:    studioName,
			ScrapedAt: now,
		}

		if m := titleRe.FindStringSubmatch(block); m != nil {
			scene.URL = absURL(m[1])
			scene.Title = cleanText(m[2])
		}
		if scene.URL == "" {
			continue
		}

		if m := thumbRe.FindStringSubmatch(block); m != nil {
			scene.Thumbnail = absURL(m[1])
			if id := contentIDRe.FindStringSubmatch(m[1]); id != nil {
				scene.ID = id[1] // prefer the numeric content id
			}
		}

		if m := dateRe.FindStringSubmatch(block); m != nil {
			if d, err := parseutil.TryParseDate(m[1], "01/02/2006"); err == nil {
				scene.Date = d.UTC()
			}
		}

		if m := durationRe.FindStringSubmatch(block); m != nil {
			if mins, err := strconv.Atoi(m[1]); err == nil {
				scene.Duration = mins * 60
			}
		}

		seenP := make(map[string]bool)
		for _, lm := range modelLinkRe.FindAllStringSubmatch(block, -1) {
			name := strings.TrimSpace(html.UnescapeString(lm[1]))
			if name != "" && !seenP[name] {
				seenP[name] = true
				scene.Performers = append(scene.Performers, name)
			}
		}

		scenes = append(scenes, scene)
	}
	return scenes
}

func absURL(p string) string {
	p = strings.TrimSpace(html.UnescapeString(p))
	if p == "" || strings.HasPrefix(p, "http") {
		return p
	}
	return siteBase + "/" + strings.TrimPrefix(p, "/")
}

func (s *Scraper) get(ctx context.Context, u string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{URL: u, Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox)})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}

var tagStripRe = regexp.MustCompile(`<[^>]+>`)

func cleanText(s string) string {
	s = tagStripRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}
