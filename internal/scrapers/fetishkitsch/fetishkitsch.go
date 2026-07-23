// Package fetishkitsch scrapes FetishKitsch (fetishkitsch.com), a custom
// Next.js app over a MongoDB backend (ObjectId keys, BunnyCDN images, Bunny
// Stream video).
//
// The homepage renders client-side and its `pageProps` holds only promo data,
// so there is nothing to scrape from HTML. Instead the site exposes an
// unauthenticated `/api/post` endpoint that returns the **entire** catalogue —
// 246 posts, ~500 KB — in one response. There is no pagination at all: a
// `?page=` parameter is accepted and ignored, giving a byte-identical body.
//
// Values arrive underscore-separated (`Rubber_Toy_Red_Part_3`,
// `Red_August`, `Bondage_Mitts`), so titles, cast and tags are all de-slugged.
//
// Two quirks of the tag list:
//
//   - Shoot years are mixed in as tags ("2020"), and are kept — they are how
//     the site itself files scenes.
//   - The API publishes no description field; the whole catalogue has none.
//
// The archive spans 2010–2020 and looks dormant, but is fully public.
package fetishkitsch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const (
	siteID     = "fetishkitsch"
	studioName = "FetishKitsch"
	dateLayout = "Jan 02, 2006"
	// The catalogue comes back in one ~500 KB response; the cap leaves room for
	// growth without going near httpx's default.
	maxCatalogueBytes = 32 * 1024 * 1024
)

var siteBase = "https://fetishkitsch.com"

// Scraper implements scraper.StudioScraper for FetishKitsch.
type Scraper struct {
	Client *http.Client
}

// New constructs a FetishKitsch scraper.
func New() *Scraper {
	return &Scraper{Client: httpx.NewClient(60 * time.Second)}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		"fetishkitsch.com",
		"fetishkitsch.com/post/{id}",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?fetishkitsch\.com(?:/|$)`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, _ scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, out)
	return out, nil
}

// ---- API ----

type apiPost struct {
	ID             string   `json:"_id"`
	Title          string   `json:"title"`
	People         []string `json:"people"`
	Tags           []string `json:"tags"`
	PublishDate    string   `json:"publishDate"`
	ShootDate      string   `json:"shootDate"`
	VideoLength    int      `json:"videoLength"`
	VideoThumbnail string   `json:"videoThumbnail"`
	Images         []string `json:"images"`
	Public         bool     `json:"public"`
}

type apiResponse struct {
	Posts []apiPost `json:"posts"`
}

// fetchCatalogue returns every post. The endpoint has no pagination — `?page=`
// is accepted and ignored — so this is a single request for the whole site.
func (s *Scraper) fetchCatalogue(ctx context.Context) ([]apiPost, error) {
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL:     siteBase + "/api/post",
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := httpx.ReadBodyN(resp.Body, maxCatalogueBytes)
	if err != nil {
		return nil, fmt.Errorf("reading catalogue: %w", err)
	}

	var ar apiResponse
	if err := json.Unmarshal(body, &ar); err != nil {
		return nil, fmt.Errorf("parsing catalogue: %w", err)
	}
	return ar.Posts, nil
}

// ---- runner ----

func (s *Scraper) run(ctx context.Context, studioURL string, out chan<- scraper.SceneResult) {
	defer close(out)

	posts, err := s.fetchCatalogue(ctx)
	if err != nil {
		select {
		case out <- scraper.Error(err):
		case <-ctx.Done():
		}
		return
	}
	scraper.Debugf(1, "%s: %d posts in the catalogue", siteID, len(posts))

	select {
	case out <- scraper.Progress(len(posts)):
	case <-ctx.Done():
		return
	}

	now := time.Now().UTC()
	for _, p := range posts {
		if !p.Public {
			continue
		}
		select {
		case out <- scraper.Scene(toScene(studioURL, p, now)):
		case <-ctx.Done():
			return
		}
	}
}

func toScene(studioURL string, p apiPost, now time.Time) models.Scene {
	scene := models.Scene{
		ID:        p.ID,
		SiteID:    siteID,
		StudioURL: studioURL,
		Title:     deslug(p.Title),
		URL:       siteBase + "/post/" + p.ID,
		Duration:  p.VideoLength,
		Thumbnail: p.VideoThumbnail,
		Studio:    studioName,
		ScrapedAt: now,
	}
	if scene.Thumbnail == "" && len(p.Images) > 0 {
		scene.Thumbnail = p.Images[0]
	}
	// publishDate is when the scene went live; shootDate is when it was filmed
	// and is the fallback for the handful of posts without one.
	if t, ok := parseDate(p.PublishDate); ok {
		scene.Date = t
	} else if t, ok := parseDate(p.ShootDate); ok {
		scene.Date = t
	}
	scene.Performers = deslugAll(p.People)
	scene.Tags = deslugAll(p.Tags)
	return scene
}

func parseDate(s string) (time.Time, bool) {
	t, err := time.Parse(dateLayout, strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

// deslug turns the API's underscore-separated values into readable text:
// "Rubber_Toy_Red_Part_3" → "Rubber Toy Red Part 3".
func deslug(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "_", " ")), " ")
}

func deslugAll(vals []string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, v := range vals {
		d := deslug(v)
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	return out
}
