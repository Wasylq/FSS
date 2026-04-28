package taratainton

import (
	"context"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/internal/scrapers/wputil"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
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
		headers:  wputil.BrowserHeaders(),
	}
}

func init() {
	scraper.Register(New())
}

func (s *Scraper) ID() string { return "taratainton" }

func (s *Scraper) Patterns() []string {
	return []string{"taratainton.com"}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?taratainton\.com`)

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
			[]string{
				s.siteBase + "/post-sitemap.xml",
				s.siteBase + "/post-sitemap2.xml",
			},
			studioURL, opts, parsePage, out)
	}()
	return out, nil
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
