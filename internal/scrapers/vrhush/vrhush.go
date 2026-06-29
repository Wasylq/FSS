// Package vrhush scrapes VRHush (vrhush.com), a Next.js VR site. The scene
// catalog is exposed through the Next.js data API: the homepage embeds a
// buildId in its <script id="__NEXT_DATA__"> block, and the listing for each
// page lives at /_next/data/{buildId}/scenes.json?page={N} under
// pageProps.contents = {total, total_pages, data[]}. The runner reads the
// buildId once, then paginates the data API to total_pages.
package vrhush

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

const (
	siteID     = "vrhush"
	studioName = "VRHush"
)

// siteBase is a var (not const) so tests can point it at a local httptest server.
var siteBase = "https://vrhush.com"

// Scraper implements scraper.StudioScraper for VRHush.
type Scraper struct {
	Client *http.Client
}

// New constructs a VRHush scraper.
func New() *Scraper {
	return &Scraper{Client: httpx.NewClient(30 * time.Second)}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		"vrhush.com",
		"vrhush.com/scenes/{slug}",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?vrhush\.com`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// ---- JSON shapes ----

type scenesPage struct {
	PageProps struct {
		Contents struct {
			Total      int     `json:"total"`
			TotalPages int     `json:"total_pages"`
			Data       []video `json:"data"`
		} `json:"contents"`
	} `json:"pageProps"`
}

type video struct {
	ID             int      `json:"id"`
	Title          string   `json:"title"`
	Slug           string   `json:"slug"`
	PublishDate    string   `json:"publish_date"`
	VideosDuration string   `json:"videos_duration"`
	Tags           []string `json:"tags"`
	Models         []string `json:"models"`
	Description    string   `json:"description"`
	Thumbnail      string   `json:"thumbnail"`
	Views          int      `json:"views"`
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()

	scraper.Debugf(1, "%s: resolving buildId", siteID)
	buildID, err := s.fetchBuildID(ctx)
	if err != nil {
		select {
		case out <- scraper.Error(fmt.Errorf("buildId: %w", err)):
		case <-ctx.Done():
		}
		return
	}
	scraper.Debugf(1, "%s: buildId=%s", siteID, buildID)

	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		vids, total, totalPages, err := s.fetchPage(ctx, buildID, page)
		if err != nil {
			return scraper.PageResult{}, err
		}
		scenes := make([]models.Scene, len(vids))
		for i, v := range vids {
			scenes[i] = toScene(studioURL, v, now)
		}
		return scraper.PageResult{Scenes: scenes, Total: total, Done: page >= totalPages}, nil
	})
}

var nextDataRe = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__" type="application/json">(.*?)</script>`)
var buildIDRe = regexp.MustCompile(`"buildId"\s*:\s*"([^"]+)"`)

func (s *Scraper) fetchBuildID(ctx context.Context) (string, error) {
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL:     siteBase + "/",
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return "", err
	}
	body, err := func() ([]byte, error) {
		defer func() { _ = resp.Body.Close() }()
		return httpx.ReadBody(resp.Body)
	}()
	if err != nil {
		return "", err
	}
	// Prefer the buildId inside the __NEXT_DATA__ block, fall back to any
	// "buildId":"…" occurrence in the page.
	search := body
	if m := nextDataRe.FindSubmatch(body); m != nil {
		search = m[1]
	}
	if m := buildIDRe.FindSubmatch(search); m != nil {
		return string(m[1]), nil
	}
	return "", fmt.Errorf("buildId not found in homepage")
}

func (s *Scraper) fetchPage(ctx context.Context, buildID string, page int) ([]video, int, int, error) {
	u := fmt.Sprintf("%s/_next/data/%s/scenes.json?page=%d", siteBase, buildID, page)
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL:     u,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, 0, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	var sp scenesPage
	if err := httpx.DecodeJSON(resp.Body, &sp); err != nil {
		return nil, 0, 0, fmt.Errorf("decode: %w", err)
	}
	c := sp.PageProps.Contents
	return c.Data, c.Total, c.TotalPages, nil
}

func toScene(studioURL string, v video, now time.Time) models.Scene {
	url := siteBase
	if v.Slug != "" {
		url = fmt.Sprintf("%s/scenes/%s", siteBase, v.Slug)
	}

	scene := models.Scene{
		ID:          strconv.Itoa(v.ID),
		SiteID:      siteID,
		StudioURL:   studioURL,
		Title:       html.UnescapeString(strings.TrimSpace(v.Title)),
		URL:         url,
		Description: html.UnescapeString(strings.TrimSpace(v.Description)),
		Studio:      studioName,
		Date:        parseDate(v.PublishDate),
		Duration:    parseDuration(v.VideosDuration),
		Views:       v.Views,
		ScrapedAt:   now,
	}

	for _, m := range v.Models {
		if n := strings.TrimSpace(m); n != "" {
			scene.Performers = append(scene.Performers, n)
		}
	}
	for _, t := range v.Tags {
		if t = strings.TrimSpace(t); t != "" {
			scene.Tags = append(scene.Tags, t)
		}
	}

	if thumb := strings.TrimSpace(v.Thumbnail); thumb != "" {
		if strings.HasPrefix(thumb, "//") {
			thumb = "https:" + thumb
		}
		scene.Thumbnail = thumb
	}

	return scene
}

// parseDate parses publish_date values like "2026/06/23 00:00:00".
func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse("2006/01/02 15:04:05", s); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

// parseDuration parses videos_duration, a float string of seconds like
// "2527.12", and returns whole seconds.
func parseDuration(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil && f > 0 {
		return int(f)
	}
	return 0
}
