package taratainton

import (
	"context"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Anastylosis/FSS/internal/httpx"
	"github.com/Anastylosis/FSS/internal/scrapers/wputil"
	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/scraper"
)

type Scraper struct {
	client   *http.Client
	siteBase string
	headers  map[string]string
}

func New() *Scraper {
	return &Scraper{
		client:   httpx.NewClient(30 * time.Second),
		siteBase: "https://www.taratainton.com",
		headers:  httpx.BrowserHeaders(httpx.UserAgentFirefox),
	}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() {
	scraper.Register(New())
}

func (s *Scraper) ID() string { return "taratainton" }

func (s *Scraper) Patterns() []string {
	return []string{"taratainton.com"}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?taratainton\.com(?:/|$)`)

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	if opts.Workers <= 0 {
		opts.Workers = 3
	}
	out := make(chan scraper.SceneResult)
	go func() {
		defer close(out)
		wputil.RunWorkerPool(ctx, s.client, s.headers,
			s.postSitemaps(ctx), studioURL, opts, parsePage, out)
	}()
	return out, nil
}

// postSitemaps resolves the post sitemaps from the site's sitemap index, so a
// third file appearing does not silently truncate the catalogue. Falls back to
// the two known files if the index cannot be read — losing the index is not a
// reason to scrape nothing.
func (s *Scraper) postSitemaps(ctx context.Context) []string {
	fallback := []string{
		s.siteBase + "/post-sitemap.xml",
		s.siteBase + "/post-sitemap2.xml",
	}

	locs, err := wputil.FetchSitemapIndex(ctx, s.client, s.siteBase+"/sitemap_index.xml", s.headers)
	if err != nil {
		scraper.Debugf(1, "taratainton: sitemap index unavailable (%v), using %d known sitemaps", err, len(fallback))
		return fallback
	}

	var posts []string
	for _, loc := range locs {
		if strings.Contains(loc, "post-sitemap") {
			posts = append(posts, loc)
		}
	}
	if len(posts) == 0 {
		scraper.Debugf(1, "taratainton: sitemap index listed no post sitemaps, using %d known", len(fallback))
		return fallback
	}
	scraper.Debugf(1, "taratainton: %d post sitemaps from the index", len(posts))
	return posts
}

// ---- site-specific parsing ----

var (
	priceLengthRe = regexp.MustCompile(`Price:\s*\$([0-9.]+)(?:&nbsp;|\s)*Length:\s*([0-9:]+)`)
	resolutionRe  = regexp.MustCompile(`(\d{3,4})p`)
	tagRe         = regexp.MustCompile(`<a\s+href="https?://(?:www\.)?taratainton\.com/tag/[^"]*"[^>]*>([^<]+)</a>`)
)

const titleSuffix = " - Tara Tainton"

func parsePage(studioURL, pageURL string, body []byte, now time.Time) (models.Scene, bool, error) {
	plMatch := priceLengthRe.FindSubmatch(body)
	if plMatch == nil {
		return models.Scene{}, true, nil
	}

	meta := wputil.ParseMeta(body, titleSuffix)

	id := meta.PostID
	if id == "" {
		id = wputil.SlugFromURL(pageURL)
	}

	price, priceErr := strconv.ParseFloat(string(plMatch[1]), 64)
	duration := wputil.ParseDuration(string(plMatch[2]))

	resolution := ""
	var width, height int
	if m := resolutionRe.FindSubmatch(body); m != nil {
		h, _ := strconv.Atoi(string(m[1]))
		height = h
		width = wputil.VideoWidth(h)
		resolution = string(m[1]) + "p"
	}

	var tags []string
	seen := make(map[string]bool)
	for _, m := range tagRe.FindAllSubmatch(body, -1) {
		tag := html.UnescapeString(strings.TrimSpace(string(m[1])))
		if !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
		}
	}

	scene := models.Scene{
		ID:          id,
		SiteID:      "taratainton",
		StudioURL:   studioURL,
		Title:       meta.Title,
		URL:         pageURL,
		Date:        meta.Date,
		Description: meta.Description,
		Thumbnail:   meta.Thumbnail,
		Performers:  []string{"Tara Tainton"},
		Studio:      "Tara Tainton",
		Tags:        tags,
		Duration:    duration,
		Resolution:  resolution,
		Width:       width,
		Height:      height,
		ScrapedAt:   now,
	}

	if priceErr == nil {
		scene.AddPrice(models.PriceSnapshot{
			Date:    now,
			Regular: price,
		})
	}

	return scene, false, nil
}
