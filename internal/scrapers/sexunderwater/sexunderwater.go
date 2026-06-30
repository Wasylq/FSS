// Package sexunderwater scrapes Sex Underwater (sexunderwater.com), an
// Elevated X tour. The SexUnderwater_{N}_d.html listing carries everything
// on the card: the title, the featured model(s), the publish date, a trailer
// mp4 (which doubles as the stable scene id and URL) and a thumbnail. There
// is no separate detail page worth fetching, so the runner parses the listing
// directly. The "Underwater Glamour" category lives on the same site and is
// covered by the same listing.
package sexunderwater

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

const (
	siteID     = "sexunderwater"
	studioName = "Sex Underwater"
	perPage    = 10
)

var siteBase = "https://sexunderwater.com"

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
		"sexunderwater.com",
		"sexunderwater.com/categories/SexUnderwater_{n}_d.html",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?sexunderwater\.com`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	itemSplitRe = regexp.MustCompile(`<div class="updateItem">`)
	trailerRe   = regexp.MustCompile(`tload\('/trailers/([^']+\.mp4)'\)`)
	titleRe     = regexp.MustCompile(`(?s)<h4>.*?<a[^>]*>(.*?)</a>`)
	modelsBlkRe = regexp.MustCompile(`(?s)tour_update_models"[^>]*>(.*?)</span>`)
	modelLinkRe = regexp.MustCompile(`<a[^>]*>([^<]+)</a>`)
	dateRe      = regexp.MustCompile(`(\d{2}/\d{2}/\d{4})`)
	thumbRe     = regexp.MustCompile(`src0_1x="([^"]+)"`)
)

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	seen := make(map[string]bool)
	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := fmt.Sprintf("%s/categories/SexUnderwater_%d_d.html", siteBase, page)
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
		return scraper.PageResult{Scenes: fresh, Done: len(scenes) < perPage}, nil
	})
}

func parseListing(body []byte, studioURL string, now time.Time) []models.Scene {
	page := string(body)
	locs := itemSplitRe.FindAllStringIndex(page, -1)
	scenes := make([]models.Scene, 0, len(locs))
	for i, loc := range locs {
		start := loc[0]
		end := len(page)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		block := page[start:end]

		tm := trailerRe.FindStringSubmatch(block)
		if tm == nil {
			continue
		}
		file := tm[1] // e.g. lunch_break-60_su-tr.mp4
		id := strings.TrimSuffix(file, ".mp4")

		scene := models.Scene{
			ID:        id,
			SiteID:    siteID,
			StudioURL: studioURL,
			URL:       siteBase + "/trailers/" + file,
			Studio:    studioName,
			ScrapedAt: now,
		}

		if m := titleRe.FindStringSubmatch(block); m != nil {
			scene.Title = cleanText(m[1])
		}
		if scene.Title == "" {
			scene.Title = id
		}

		if m := modelsBlkRe.FindStringSubmatch(block); m != nil {
			seen := make(map[string]bool)
			for _, lm := range modelLinkRe.FindAllStringSubmatch(m[1], -1) {
				name := strings.TrimSpace(html.UnescapeString(lm[1]))
				if name != "" && !seen[name] {
					seen[name] = true
					scene.Performers = append(scene.Performers, name)
				}
			}
		}

		if m := dateRe.FindStringSubmatch(block); m != nil {
			if d, err := parseutil.TryParseDate(m[1], "01/02/2006"); err == nil {
				scene.Date = d.UTC()
			}
		}

		if m := thumbRe.FindStringSubmatch(block); m != nil {
			scene.Thumbnail = absURL(m[1])
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
