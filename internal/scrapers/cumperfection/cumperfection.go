// Package cumperfection scrapes Cum Perfection (cumperfection.com), an ElevatedX
// "Classic" template site. Every scene's metadata (title, performers, duration,
// date, thumbnail) is published on the listing cards, but the cards themselves
// link to /join/ — there is no public per-scene detail page. The scene's URL is
// therefore synthesised from the featured performer's model page (the only real,
// reachable per-scene-ish URL the public tour exposes); scenes with no performer
// fall back to the studio root.
package cumperfection

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"

	"net/http"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const (
	siteID     = "cumperfection"
	studioName = "Cum Perfection"
	siteBase   = "https://www.cumperfection.com"
	perPage    = 28
)

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?cumperfection\.com(?:/|$)`)

type Scraper struct {
	client *http.Client
}

func New() *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second)}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		"cumperfection.com",
		"cumperfection.com/categories/movies.html",
		"cumperfection.com/models/{slug}.html",
	}
}

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	// Model page: a single page of the same card format, no pagination.
	if strings.Contains(studioURL, "/models/") {
		scraper.Debugf(1, "cumperfection: scraping model page %s", studioURL)
		body, err := s.fetchPage(ctx, studioURL)
		if err != nil {
			select {
			case out <- scraper.Error(err):
			case <-ctx.Done():
			}
			return
		}
		scenes := parseListing(body, studioURL)
		select {
		case out <- scraper.Progress(len(scenes)):
		case <-ctx.Done():
			return
		}
		for _, sc := range scenes {
			if opts.KnownIDs[sc.ID] {
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
				return
			}
			select {
			case out <- scraper.Scene(sc):
			case <-ctx.Done():
				return
			}
		}
		return
	}

	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := fmt.Sprintf("%s/categories/movies_%d_d.html", siteBase, page)
		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}
		scenes := parseListing(body, studioURL)
		var total int
		if page == 1 {
			total = estimateTotal(body)
		}
		return scraper.PageResult{
			Scenes: scenes,
			Total:  total,
			Done:   len(scenes) < perPage,
		}, nil
	})
}

var (
	blockRe     = regexp.MustCompile(`class="update_details" data-setid="(\d+)"`)
	titleRe     = regexp.MustCompile(`(?s)<!-- Title -->\s*<a[^>]*>\s*(.*?)\s*</a>`)
	modelRe     = regexp.MustCompile(`href="([^"]*/models/[^"]+\.html)"[^>]*>([^<]+)</a>`)
	durationRe  = regexp.MustCompile(`(\d+)\s*(?:&nbsp;)*\s*minute`)
	dateBlockRe = regexp.MustCompile(`(?s)class="[^"]*update_date[^"]*">(.*?)</div>`)
	dateValRe   = regexp.MustCompile(`[A-Z][a-z]+ \d{1,2}, \d{4}`)
	thumbRe     = regexp.MustCompile(`data-src0_1x="([^"]+)"`)
	pageOfRe    = regexp.MustCompile(`Page \d+ of (\d+)`)
)

// parseListing extracts every scene card from a listing/model page.
func parseListing(body []byte, studioURL string) []models.Scene {
	page := string(body)
	locs := blockRe.FindAllStringSubmatchIndex(page, -1)
	scenes := make([]models.Scene, 0, len(locs))
	now := time.Now().UTC()

	for i, loc := range locs {
		id := page[loc[2]:loc[3]]
		start := loc[0]
		end := len(page)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		block := page[start:end]

		scene := models.Scene{
			ID:        id,
			SiteID:    siteID,
			StudioURL: studioURL,
			Studio:    studioName,
			ScrapedAt: now,
		}

		if m := titleRe.FindStringSubmatch(block); m != nil {
			scene.Title = strings.TrimSpace(html.UnescapeString(m[1]))
		}

		var firstModelURL string
		for _, m := range modelRe.FindAllStringSubmatch(block, -1) {
			modelURL, name := m[1], strings.TrimSpace(html.UnescapeString(m[2]))
			if name == "" {
				continue
			}
			scene.Performers = append(scene.Performers, name)
			if firstModelURL == "" {
				firstModelURL = absURL(modelURL)
			}
		}

		// No public per-scene page — point URL at the featured model page,
		// else the studio root.
		if firstModelURL != "" {
			scene.URL = firstModelURL
		} else {
			scene.URL = siteBase + "/"
		}

		if m := durationRe.FindStringSubmatch(block); m != nil {
			mins, _ := strconv.Atoi(m[1])
			scene.Duration = mins * 60
		}

		if m := dateBlockRe.FindStringSubmatch(block); m != nil {
			if dm := dateValRe.FindString(m[1]); dm != "" {
				if t, err := time.Parse("January 2, 2006", dm); err == nil {
					scene.Date = t.UTC()
				}
			}
		}

		if m := thumbRe.FindStringSubmatch(block); m != nil {
			scene.Thumbnail = absURL(m[1])
		}

		if scene.Title == "" {
			continue
		}
		scenes = append(scenes, scene)
	}
	return scenes
}

func estimateTotal(body []byte) int {
	if m := pageOfRe.FindSubmatch(body); m != nil {
		if n, _ := strconv.Atoi(string(m[1])); n > 0 {
			return n * perPage
		}
	}
	return 0
}

func absURL(u string) string {
	if strings.HasPrefix(u, "/") {
		return siteBase + u
	}
	return u
}

func (s *Scraper) fetchPage(ctx context.Context, url string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     url,
		Headers: httpx.BrowserHeaders(httpx.UserAgentChrome),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}
