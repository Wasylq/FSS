// Package manipulativemedia scrapes the Manipulative Media network — My Pervy
// Family (mypervyfamily.com) and Touch My Wife (touchmywife.com). Both sites run
// on the DTI / project1service.com tour platform.
//
// The server-rendered HTML is a JS app shell and is not reliably paginated, but
// the platform exposes a JSON catalog at site-api.project1service.com/v2/releases.
// That endpoint requires an `Instance` JWT header, which the tour hands out for
// free as the `instance_token` cookie on any page load. We bootstrap that token
// from the site homepage, then page the releases endpoint (orderBy=-dateReleased,
// type=scene) to enumerate the full catalog with rich metadata: title, release
// date, description, performers, tags, duration, poster, and view/like stats.
package manipulativemedia

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

// apiBase is a var (not const) so tests can point it at an httptest server.
var apiBase = "https://site-api.project1service.com/v2/releases"

const pageSize = 100

// site holds the per-brand configuration for a single network member.
type site struct {
	id      string // stable scraper ID / SiteID, e.g. "mypervyfamily"
	name    string // human studio name, e.g. "My Pervy Family"
	base    string // tour base URL, e.g. "https://www.mypervyfamily.com"
	matchRe *regexp.Regexp
}

// Scraper implements scraper.StudioScraper for one Manipulative Media site.
type Scraper struct {
	site   site
	client *http.Client
}

func newScraper(s site) *Scraper {
	// A cookie jar lets the homepage bootstrap carry the instance_token, though
	// we read it explicitly from the response too.
	c := httpx.NewClient(30 * time.Second)
	if jar, err := cookiejar.New(nil); err == nil {
		c.Jar = jar
	}
	return &Scraper{site: s, client: c}
}

// NewMyPervyFamily returns the My Pervy Family scraper.
func NewMyPervyFamily() *Scraper {
	return newScraper(site{
		id:      "mypervyfamily",
		name:    "My Pervy Family",
		base:    "https://www.mypervyfamily.com",
		matchRe: regexp.MustCompile(`^https?://(?:www\.)?mypervyfamily\.com`),
	})
}

// NewTouchMyWife returns the Touch My Wife scraper.
func NewTouchMyWife() *Scraper {
	return newScraper(site{
		id:      "touchmywife",
		name:    "Touch My Wife",
		base:    "https://www.touchmywife.com",
		matchRe: regexp.MustCompile(`^https?://(?:www\.)?touchmywife\.com`),
	})
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() {
	scraper.Register(NewMyPervyFamily())
	scraper.Register(NewTouchMyWife())
}

func (s *Scraper) ID() string { return s.site.id }

func (s *Scraper) Patterns() []string {
	host := strings.TrimPrefix(s.site.base, "https://www.")
	return []string{host, host + "/videos", host + "/video/{id}/{slug}"}
}

func (s *Scraper) MatchesURL(u string) bool { return s.site.matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// ---- API response shapes ----

type apiResponse struct {
	Meta struct {
		Count int `json:"count"`
		Total int `json:"total"`
	} `json:"meta"`
	Result []release `json:"result"`
}

type release struct {
	ID           int    `json:"id"`
	Title        string `json:"title"`
	DateReleased string `json:"dateReleased"`
	Description  string `json:"description"`
	Actors       []struct {
		Name string `json:"name"`
	} `json:"actors"`
	Tags []struct {
		Name      string `json:"name"`
		IsVisible bool   `json:"isVisible"`
	} `json:"tags"`
	Videos struct {
		Mediabook struct {
			Length int `json:"length"`
		} `json:"mediabook"`
	} `json:"videos"`
	// Images.Poster maps a poster index ("0", "1", …) to a map of size labels
	// (xs/sm/md/lg/xl/xx) → an object carrying the image URL. The same object
	// also carries non-image metadata keys (alternateText, imageVersion) whose
	// values are strings/ints, so each entry is decoded lazily.
	Images struct {
		Poster map[string]json.RawMessage `json:"poster"`
	} `json:"images"`
	Stats struct {
		Views int `json:"views"`
		Likes int `json:"likes"`
	} `json:"stats"`
}

// ---- runner ----

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	scraper.Debugf(1, "%s: bootstrapping instance_token from %s", s.site.id, s.site.base)
	token, err := s.fetchToken(ctx)
	if err != nil {
		select {
		case out <- scraper.Error(fmt.Errorf("%s: bootstrap token: %w", s.site.id, err)):
		case <-ctx.Done():
		}
		return
	}

	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, s.site.id, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		offset := (page - 1) * pageSize
		resp, total, err := s.fetchPage(ctx, token, offset)
		if err != nil {
			return scraper.PageResult{}, err
		}
		scenes := make([]models.Scene, 0, len(resp))
		for _, rel := range resp {
			scenes = append(scenes, s.toScene(studioURL, rel, now))
		}
		done := len(resp) < pageSize || offset+len(resp) >= total
		return scraper.PageResult{Scenes: scenes, Total: total, Done: done}, nil
	})
}

// fetchToken loads the tour homepage and extracts the instance_token cookie,
// which authorizes the site-api releases endpoint.
func (s *Scraper) fetchToken(ctx context.Context) (string, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     s.site.base + "/",
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	for _, c := range resp.Cookies() {
		if c.Name == "instance_token" && c.Value != "" {
			return c.Value, nil
		}
	}
	return "", fmt.Errorf("instance_token cookie not found")
}

// fetchPage queries one page of releases and returns the releases plus the total
// catalog count.
func (s *Scraper) fetchPage(ctx context.Context, token string, offset int) ([]release, int, error) {
	u := fmt.Sprintf("%s?limit=%d&offset=%d&type=scene&orderBy=-dateReleased", apiBase, pageSize, offset)
	headers := map[string]string{
		"User-Agent": httpx.UserAgentFirefox,
		"Accept":     "application/json",
		"Instance":   token,
		"Origin":     s.site.base,
		"Referer":    s.site.base + "/",
	}
	resp, err := httpx.Do(ctx, s.client, httpx.Request{URL: u, Headers: headers})
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	var data apiResponse
	if err := httpx.DecodeJSON(resp.Body, &data); err != nil {
		return nil, 0, fmt.Errorf("decoding releases: %w", err)
	}
	return data.Result, data.Meta.Total, nil
}

// ---- mapping ----

func (s *Scraper) toScene(studioURL string, rel release, now time.Time) models.Scene {
	scene := models.Scene{
		ID:          strconv.Itoa(rel.ID),
		SiteID:      s.site.id,
		StudioURL:   studioURL,
		Title:       strings.TrimSpace(rel.Title),
		URL:         fmt.Sprintf("%s/video/%d/%s", s.site.base, rel.ID, slugify(rel.Title)),
		Description: strings.TrimSpace(rel.Description),
		Studio:      s.site.name,
		Duration:    rel.Videos.Mediabook.Length,
		Thumbnail:   bestPoster(rel),
		Views:       rel.Stats.Views,
		Likes:       rel.Stats.Likes,
		ScrapedAt:   now,
	}

	if t, err := time.Parse(time.RFC3339, rel.DateReleased); err == nil {
		scene.Date = t.UTC()
	}

	for _, a := range rel.Actors {
		if n := strings.TrimSpace(a.Name); n != "" {
			scene.Performers = append(scene.Performers, n)
		}
	}
	for _, tg := range rel.Tags {
		if tg.IsVisible {
			if n := strings.TrimSpace(tg.Name); n != "" {
				scene.Tags = append(scene.Tags, n)
			}
		}
	}
	scene.AddPrice(models.PriceSnapshot{Date: now, IsFree: true})
	return scene
}

// posterSizes lists poster size labels from largest to smallest preference.
var posterSizes = []string{"xx", "xl", "lg", "md", "sm", "xs"}

// bestPoster returns the highest-resolution poster URL available for a release.
// The poster map keys numeric indices ("0", "1", …) alongside metadata keys
// (alternateText, imageVersion); only the lowest numeric index holds size data.
func bestPoster(rel release) string {
	if len(rel.Images.Poster) == 0 {
		return ""
	}
	best := -1
	var raw json.RawMessage
	for k, v := range rel.Images.Poster {
		n, err := strconv.Atoi(k)
		if err != nil {
			continue // skip alternateText / imageVersion
		}
		if best == -1 || n < best {
			best, raw = n, v
		}
	}
	if raw == nil {
		return ""
	}
	var sizes map[string]struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(raw, &sizes); err != nil {
		return ""
	}
	for _, sz := range posterSizes {
		if img, ok := sizes[sz]; ok && img.URL != "" {
			return img.URL
		}
	}
	return ""
}

var slugCleanRe = regexp.MustCompile(`[^a-z0-9]+`)

// slugify converts a title into the hyphenated slug used in canonical /video URLs.
func slugify(title string) string {
	s := strings.ToLower(title)
	s = slugCleanRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
