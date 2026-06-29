// Package bondagejunkies scrapes Bondage Junkies (bondagejunkies.com), a
// custom PHP tour. The /updates listing is fully server-rendered: each update
// block carries the id, title, tags, publish date, photo/runtime byline,
// synopsis and a preview poster, so no detail-page fetch is needed. Scenes are
// anonymous (no per-performer credit on the tour), so Performers is left empty
// and the Studio carries the identity.
package bondagejunkies

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

var siteBase = "https://bondagejunkies.com"

const siteID = "bondagejunkies"

type Scraper struct{ Client *http.Client }

func New() *Scraper { return &Scraper{Client: httpx.NewClient(30 * time.Second)} }

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }
func (s *Scraper) Patterns() []string {
	return []string{"bondagejunkies.com", "bondagejunkies.com/updates"}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?bondagejunkies\.com`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	anchorRe    = regexp.MustCompile(`<a name="(\d+)"></a>`)
	titleRe     = regexp.MustCompile(`(?s)<h1 class="updatetitle">(.*?)</h1>`)
	tagsBlockRe = regexp.MustCompile(`(?s)<p class="titletags">(.*?)</p>`)
	tagRe       = regexp.MustCompile(`<a[^>]*>([^<]+)</a>`)
	bylinerRe   = regexp.MustCompile(`(?s)<p class="byliner"[^>]*>(.*?)</p>`)
	dateRe      = regexp.MustCompile(`(\d{4}-\d{2}-\d{2})`)
	durationRe  = regexp.MustCompile(`(\d+)\s*min`)
	descRe      = regexp.MustCompile(`(?s)<p class="updatedesc">(.*?)</p>`)
	tagStripRe  = regexp.MustCompile(`<[^>]+>`)
)

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := fmt.Sprintf("%s/updates?page=%d", siteBase, page)
		body, err := s.get(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}
		scenes := parseListing(string(body), studioURL, now)
		return scraper.PageResult{Scenes: scenes}, nil
	})
}

// parseListing splits the page into per-update blocks (delimited by the
// `<a name="ID"></a>` anchors) and parses each into a Scene.
func parseListing(doc, studioURL string, now time.Time) []models.Scene {
	idx := anchorRe.FindAllStringSubmatchIndex(doc, -1)
	scenes := make([]models.Scene, 0, len(idx))
	for i, m := range idx {
		id := doc[m[2]:m[3]]
		start := m[1]
		end := len(doc)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		if sc, ok := toScene(id, doc[start:end], studioURL, now); ok {
			scenes = append(scenes, sc)
		}
	}
	return scenes
}

func toScene(id, block, studioURL string, now time.Time) (models.Scene, bool) {
	sc := models.Scene{
		ID:        id,
		SiteID:    siteID,
		StudioURL: studioURL,
		URL:       siteBase + "/updates#" + id,
		Thumbnail: siteBase + "/images/preview/" + id + "-lg-1.jpg",
		Studio:    "Bondage Junkies",
		ScrapedAt: now,
	}
	if m := titleRe.FindStringSubmatch(block); m != nil {
		sc.Title = cleanText(m[1])
	}
	if sc.Title == "" {
		return models.Scene{}, false
	}
	if m := tagsBlockRe.FindStringSubmatch(block); m != nil {
		for _, t := range tagRe.FindAllStringSubmatch(m[1], -1) {
			tag := cleanText(t[1])
			if tag != "" {
				sc.Tags = append(sc.Tags, tag)
			}
		}
	}
	if m := bylinerRe.FindStringSubmatch(block); m != nil {
		byline := m[1]
		if d := dateRe.FindStringSubmatch(byline); d != nil {
			if t, err := time.Parse("2006-01-02", d[1]); err == nil {
				sc.Date = t.UTC()
			}
		}
		if dur := durationRe.FindStringSubmatch(byline); dur != nil {
			if mins, err := strconv.Atoi(dur[1]); err == nil {
				sc.Duration = mins * 60
			}
		}
	}
	if m := descRe.FindStringSubmatch(block); m != nil {
		sc.Description = cleanText(m[1])
	}
	return sc, true
}

func (s *Scraper) get(ctx context.Context, u string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{URL: u, Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox)})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}

func cleanText(s string) string {
	s = tagStripRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}
