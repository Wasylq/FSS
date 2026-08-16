// Package porndudecasting scrapes porndudecasting.com.
//
// The site publishes a Google video sitemap that is a complete scene record —
// title, description, duration, thumbnail, publication date, rating, view
// count, category and tags — so there is nothing a detail fetch would add and
// the whole catalogue costs two requests.
//
// Each entry's `<loc>` is a `/models/{slug}/` page: on a casting site the model
// page *is* the scene page, and all 295 entries have a distinct one. The scene
// id comes from the media path (`videos_screenshots/0/846/preview.jpg` → 846),
// which is the site's own numbering; the slug would collapse a model's scenes
// together if one ever gained a second.
package porndudecasting

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Anastylosis/FSS/internal/httpx"
	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/scraper"
)

const (
	siteID     = "porndudecasting"
	domain     = "porndudecasting.com"
	studioName = "Porn Dude Casting"
	// videoSitemapPath is named in /sitemap.xml's index. It is requested
	// directly rather than discovered, because the index also lists an "other"
	// sitemap of site pages that carries no scenes.
	videoSitemapPath = "/sitemap/?type=videos&from_links_videos=1"
)

type Scraper struct {
	Client       *http.Client
	baseOverride string
}

func New() *Scraper {
	return &Scraper{Client: httpx.NewClient(60 * time.Second)}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?porndudecasting\.com(?:/|$)`)

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		domain,
		domain + "/latest-updates/",
		domain + "/models/{slug}/",
	}
}

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// base resolves the origin to fetch from. The operator's own host wins so a
// non-www spelling stays addressable; baseOverride is the test server.
func (s *Scraper) base(studioURL string) string {
	if s.baseOverride != "" {
		return strings.TrimSuffix(s.baseOverride, "/")
	}
	if u, err := url.Parse(studioURL); err == nil && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}
	return "https://" + domain
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	base := s.base(studioURL)
	scraper.Debugf(1, "%s: reading the video sitemap", siteID)

	body, err := s.fetch(ctx, base+videoSitemapPath)
	if err != nil {
		send(ctx, out, scraper.Error(fmt.Errorf("video sitemap: %w", err)))
		return
	}

	var set urlSet
	if err := xml.Unmarshal(body, &set); err != nil {
		send(ctx, out, scraper.Error(scraper.ParseError(base+videoSitemapPath,
			fmt.Errorf("decoding sitemap: %w", err))))
		return
	}

	now := time.Now().UTC()
	scenes := make([]models.Scene, 0, len(set.URLs))
	seen := make(map[string]bool, len(set.URLs))
	for _, entry := range set.URLs {
		sc, ok := toScene(entry, studioURL, now)
		if !ok || seen[sc.ID] {
			continue
		}
		seen[sc.ID] = true
		scenes = append(scenes, sc)
	}

	if len(scenes) == 0 {
		// A sitemap that fetched and decoded but named no scenes is a feed
		// change, not an empty catalogue, and must not read as one to an
		// authoritative --full save.
		send(ctx, out, scraper.Error(scraper.ParseError(base+videoSitemapPath,
			fmt.Errorf("no video entries in the sitemap"))))
		return
	}

	// The feed is newest-first in practice, but sorting makes the KnownIDs stop
	// below mean what it says rather than depend on that holding.
	sort.SliceStable(scenes, func(i, j int) bool {
		if !scenes[i].Date.Equal(scenes[j].Date) {
			return scenes[i].Date.After(scenes[j].Date)
		}
		return numericID(scenes[i].ID) > numericID(scenes[j].ID)
	})

	pending := scenes
	for i, sc := range scenes {
		if opts.KnownIDs[sc.ID] {
			scraper.Debugf(1, "%s: hit known ID %s, stopping early", siteID, sc.ID)
			pending = scenes[:i]
			if !send(ctx, out, scraper.StoppedEarly()) {
				return
			}
			break
		}
	}
	if len(pending) == 0 {
		return
	}

	scraper.Debugf(1, "%s: %d scenes from the sitemap", siteID, len(pending))
	if !send(ctx, out, scraper.Progress(len(pending))) {
		return
	}
	for _, sc := range pending {
		if !send(ctx, out, scraper.Scene(sc)) {
			return
		}
	}
}

func (s *Scraper) fetch(ctx context.Context, u string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		Method:  http.MethodGet,
		URL:     u,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}

// ---- sitemap ----

type urlSet struct {
	URLs []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc   string `xml:"loc"`
	Video struct {
		ThumbnailLoc    string   `xml:"thumbnail_loc"`
		Title           string   `xml:"title"`
		Description     string   `xml:"description"`
		Duration        int      `xml:"duration"`
		ContentLoc      string   `xml:"content_loc"`
		Rating          string   `xml:"rating"`
		ViewCount       int      `xml:"view_count"`
		PublicationDate string   `xml:"publication_date"`
		Category        string   `xml:"category"`
		Tags            []string `xml:"tag"`
	} `xml:"video"`
}

// mediaIDRe pulls the site's own video number out of a media path. Both the
// screenshot and the download URL carry it.
var mediaIDRe = regexp.MustCompile(`/(\d+)/(?:preview|\d+)\.(?:jpg|mp4)`)

// modelSlugRe reads the performer's slug out of the entry's page URL.
var modelSlugRe = regexp.MustCompile(`/models/([^/?#]+)/?$`)

func toScene(e sitemapURL, studioURL string, now time.Time) (models.Scene, bool) {
	id := firstSubmatch(mediaIDRe, e.Video.ThumbnailLoc)
	if id == "" {
		id = firstSubmatch(mediaIDRe, e.Video.ContentLoc)
	}
	title := cleanText(e.Video.Title)
	if id == "" || title == "" {
		return models.Scene{}, false
	}

	scene := models.Scene{
		ID:          id,
		SiteID:      siteID,
		StudioURL:   studioURL,
		Title:       title,
		URL:         e.Loc,
		Studio:      studioName,
		Description: cleanText(e.Video.Description),
		Thumbnail:   e.Video.ThumbnailLoc,
		Duration:    e.Video.Duration,
		Views:       e.Video.ViewCount,
		Tags:        cleanNames(e.Video.Tags),
		ScrapedAt:   now,
	}
	if c := cleanText(e.Video.Category); c != "" {
		scene.Categories = []string{c}
	}
	if t, err := time.Parse("2006-01-02", strings.TrimSpace(e.Video.PublicationDate)); err == nil {
		scene.Date = t.UTC()
	}
	// The feed names no performer; the page URL does, and the title repeats it
	// in the site's own spelling — but only sometimes, so the slug is what is
	// relied on.
	if slug := firstSubmatch(modelSlugRe, e.Loc); slug != "" {
		scene.Performers = []string{slugToName(slug)}
	}
	return scene, true
}

// ---- helpers ----

var (
	tagStripRe = regexp.MustCompile(`<[^>]*>`)
	// The description is HTML with inline chapter markers ("06:45", "11:20")
	// rendered as their own divs; stripping tags leaves the timestamps loose in
	// the prose, so they go too.
	chapterTimeRe = regexp.MustCompile(`(?m)(^|\s)\d{1,2}:\d{2}(\s|$)`)
)

func cleanText(s string) string {
	// The sitemap CDATA holds escaped HTML, so the entities have to go before
	// the tags they encode can be stripped.
	s = html.UnescapeString(s)
	s = tagStripRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = chapterTimeRe.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, " ", " ")
	return strings.Join(strings.Fields(s), " ")
}

func cleanNames(in []string) []string {
	var out []string
	for _, v := range in {
		if n := cleanText(v); n != "" {
			out = append(out, n)
		}
	}
	return out
}

// slugToName turns `gracey-snow` into `Gracey Snow`.
func slugToName(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

func firstSubmatch(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func numericID(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func send(ctx context.Context, out chan<- scraper.SceneResult, r scraper.SceneResult) bool {
	select {
	case out <- r:
		return true
	case <-ctx.Done():
		return false
	}
}
