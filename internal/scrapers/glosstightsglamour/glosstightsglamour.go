// Package glosstightsglamour scrapes glosstightsglamour.com, a Glamose network
// site running on ElevatedX CMS. HTML listing with page_N.html pagination,
// 5 entries per page. The site wraps past the last page instead of returning
// empty pages — duplicate detection handles this.
package glosstightsglamour

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

const blockMarker = `class="update_block">`

var (
	titleRe   = regexp.MustCompile(`update_title">([^<]+)`)
	modelRe   = regexp.MustCompile(`/models/[^"]*\.html">([^<]+)`)
	dateRe    = regexp.MustCompile(`update_date">(\d{2}/\d{2}/\d{4})`)
	descRe    = regexp.MustCompile(`latest_update_description">([^<]+)`)
	thumbRe   = regexp.MustCompile(`large_update_thumb[^>]*\ssrc0_1x="([^"]+)"`)
	tagRe     = regexp.MustCompile(`/categories/[^"]*\.html">([^<]+)`)
	trailerRe = regexp.MustCompile(`tload\('([^']+)'`)
	setIDRe   = regexp.MustCompile(`set-target-(\d+)`)
	lastPgRe  = regexp.MustCompile(`/updates/page_(\d+)\.html`)
)

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	seen := make(map[string]bool)

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

		allSeen := len(scenes) > 0
		var fresh []models.Scene
		for _, sc := range scenes {
			if seen[sc.ID] {
				continue
			}
			seen[sc.ID] = true
			allSeen = false
			fresh = append(fresh, sc)
		}
		if allSeen {
			return scraper.PageResult{Done: true}, nil
		}

		total := 0
		if page == 1 {
			maxPg := parseMaxPage(body)
			if maxPg > 0 {
				total = maxPg * 5
			}
		}

		done := !hasNextPage(body, page)
		return scraper.PageResult{Scenes: fresh, Total: total, Done: done}, nil
	})
}

func findBlockStarts(text string) []int {
	var starts []int
	offset := 0
	for {
		idx := strings.Index(text[offset:], blockMarker)
		if idx < 0 {
			break
		}
		pos := offset + idx
		starts = append(starts, pos)
		offset = pos + len(blockMarker)
	}
	return starts
}

func parseListingPage(body []byte, studioURL string) []models.Scene {
	text := string(body)
	now := time.Now().UTC()

	starts := findBlockStarts(text)
	if len(starts) == 0 {
		return nil
	}

	var scenes []models.Scene
	for i, start := range starts {
		end := len(text)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		block := text[start:end]

		m := setIDRe.FindStringSubmatch(block)
		if m == nil {
			continue
		}
		id := m[1]

		scene := models.Scene{
			ID:        id,
			SiteID:    "glosstightsglamour",
			StudioURL: studioURL,
			URL:       fmt.Sprintf("%s/updates/#scene-%s", siteBase, id),
			Studio:    "Gloss Tights Glamour",
			ScrapedAt: now,
		}

		if m := titleRe.FindStringSubmatch(block); m != nil {
			scene.Title = strings.TrimSpace(html.UnescapeString(m[1]))
		}

		if m := modelRe.FindStringSubmatch(block); m != nil {
			scene.Performers = []string{strings.TrimSpace(m[1])}
		}

		if m := dateRe.FindStringSubmatch(block); m != nil {
			if t, err := time.Parse("02/01/2006", m[1]); err == nil {
				scene.Date = t.UTC()
			}
		}

		if m := descRe.FindStringSubmatch(block); m != nil {
			scene.Description = strings.TrimSpace(m[1])
		}

		if m := thumbRe.FindStringSubmatch(block); m != nil {
			thumb := m[1]
			if !strings.HasPrefix(thumb, "http") {
				thumb = siteBase + "/" + strings.TrimPrefix(thumb, "/")
			}
			scene.Thumbnail = thumb
		}

		if m := trailerRe.FindStringSubmatch(block); m != nil {
			trailer := m[1]
			if !strings.HasPrefix(trailer, "http") {
				trailer = siteBase + "/" + strings.TrimPrefix(trailer, "/")
			}
			scene.Preview = trailer
		}

		var tags []string
		for _, tm := range tagRe.FindAllStringSubmatch(block, -1) {
			tag := strings.TrimSpace(tm[1])
			if tag != "" {
				tags = append(tags, tag)
			}
		}
		scene.Tags = tags

		scenes = append(scenes, scene)
	}
	return scenes
}

func parseMaxPage(body []byte) int {
	max := 0
	for _, m := range lastPgRe.FindAllSubmatch(body, -1) {
		if n, err := strconv.Atoi(string(m[1])); err == nil && n > max {
			max = n
		}
	}
	return max
}

func hasNextPage(body []byte, current int) bool {
	for _, m := range lastPgRe.FindAllSubmatch(body, -1) {
		if n, err := strconv.Atoi(string(m[1])); err == nil && n > current {
			return true
		}
	}
	return false
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
