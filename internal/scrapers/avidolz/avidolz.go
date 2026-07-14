// Package avidolz scrapes AVIdolZ (avidolz.com), a WordPress JAV site using a
// custom "vms_videos" post type.
//
// The site is a frozen archive — nothing has been published since July 2020 —
// so a single full crawl is enough and there is nothing to poll for.
//
// Two quirks shape this scraper:
//
//   - The WP REST API returns 401, so scenes are enumerated from the
//     wp-sitemap "vms_videos" sitemap (369 entries).
//   - No page carries a publish date. The sitemap's <lastmod> is the only date
//     available, so it is used as an approximation; several entries share a
//     bulk re-save timestamp, so dates are indicative rather than exact.
//
// Category pages live in the same root namespace as scenes (/big-tits/ next to
// /seductive-masseuse-.../), so a scene URL cannot be told from a category URL
// by shape. The vms_videos sitemap is the authoritative scene list, which is
// why enumeration never walks the HTML listing.
package avidolz

import (
	"context"
	"html"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/internal/scrapers/wputil"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const (
	siteID     = "avidolz"
	studioName = "AVIdolZ"
)

var siteBase = "https://avidolz.com"

// Scraper implements scraper.StudioScraper for AVIdolZ.
type Scraper struct {
	client  *http.Client
	headers map[string]string
}

// New constructs an AVIdolZ scraper.
func New() *Scraper {
	return &Scraper{
		client:  httpx.NewClient(30 * time.Second),
		headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		"avidolz.com",
		"avidolz.com/japan-porn/",
		"avidolz.com/{slug}/",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?avidolz\.com(?:/|$)`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	if opts.Workers <= 0 {
		opts.Workers = 3
	}
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	sitemapURL := siteBase + "/wp-sitemap-posts-vms_videos-1.xml"

	// Fetch the sitemap up front so each page's <lastmod> can be carried into
	// the parse — it is the only date the site exposes.
	entries, err := wputil.FetchSitemap(ctx, s.client, sitemapURL, s.headers)
	if err != nil {
		select {
		case out <- scraper.Error(err):
		case <-ctx.Done():
		}
		return
	}
	scraper.Debugf(1, "%s: %d scenes in sitemap", siteID, len(entries))

	dates := make(map[string]time.Time, len(entries))
	for _, e := range entries {
		if t, err := time.Parse(time.RFC3339, e.LastMod); err == nil {
			dates[e.Loc] = t.UTC()
		}
	}

	parse := func(studioURL, pageURL string, body []byte, now time.Time) (models.Scene, bool, error) {
		return parsePage(studioURL, pageURL, body, now, dates[pageURL])
	}

	wputil.RunWorkerPool(ctx, s.client, s.headers,
		[]string{sitemapURL}, studioURL, opts, parse, out)
}

var (
	// The info panel gives a clean title; <title> carries a " | AvidolZ" suffix.
	panelTitleRe = regexp.MustCompile(`<strong>Title:\s*</strong>\s*([^<]+)`)
	titleTagRe   = regexp.MustCompile(`<title>([^<]*)</title>`)
	titleSuffix  = regexp.MustCompile(`\s*\|\s*AvidolZ\s*$`)
	// Cast is marked up as schema.org actor/Person, which scopes it to the
	// scene — bare /jav-models/ links also appear in "related models" blocks.
	actorRe = regexp.MustCompile(`(?s)itemprop="actor".*?itemprop="name">([^<]+)</span>`)
	genreRe = regexp.MustCompile(`itemprop="genre"[^>]*>([^<]+)</a>`)
	// "31Min 19sec"
	durationRe = regexp.MustCompile(`<strong>Duration:</strong>\s*(?:(\d+)\s*Min)?\s*(?:(\d+)\s*sec)?`)
	resRe      = regexp.MustCompile(`<strong>Resolution:</strong>\s*(\d+)x(\d+)`)
	// Anchored on the schema.org markup rather than a bare /series/ link: the
	// page also carries /series/ anchors in its related-content blocks, and the
	// real credit nests the name in a <span itemprop="name">.
	seriesRe   = regexp.MustCompile(`(?s)itemprop="partOfSeries".*?itemprop="name">([^<]+)</span>`)
	descRe     = regexp.MustCompile(`(?s)itemprop="description"[^>]*>\s*(?:<p>)?(.*?)(?:</p>|</div>)`)
	posterRe   = regexp.MustCompile(`poster="(//[^"]+|https?://[^"]+)"`)
	tagStripRe = regexp.MustCompile(`<[^>]+>`)
)

func parsePage(studioURL, pageURL string, body []byte, now, date time.Time) (models.Scene, bool, error) {
	detail := string(body)

	title := ""
	if m := panelTitleRe.FindStringSubmatch(detail); m != nil {
		title = html.UnescapeString(strings.TrimSpace(m[1]))
	}
	if title == "" {
		if m := titleTagRe.FindStringSubmatch(detail); m != nil {
			title = html.UnescapeString(strings.TrimSpace(titleSuffix.ReplaceAllString(m[1], "")))
		}
	}
	// A page with no title is a category or other non-scene page.
	if title == "" {
		return models.Scene{}, true, nil
	}

	scene := models.Scene{
		ID:         wputil.SlugFromURL(pageURL),
		SiteID:     siteID,
		StudioURL:  studioURL,
		Title:      title,
		URL:        pageURL,
		Date:       date,
		Studio:     studioName,
		Performers: dedupe(actorRe, detail),
		Categories: dedupe(genreRe, detail),
		Duration:   parseDuration(detail),
		ScrapedAt:  now,
	}

	if m := resRe.FindStringSubmatch(detail); m != nil {
		scene.Width = atoi(m[1])
		scene.Height = atoi(m[2])
		if scene.Height > 0 {
			scene.Resolution = m[2] + "p"
		}
	}
	if m := seriesRe.FindStringSubmatch(detail); m != nil {
		scene.Series = html.UnescapeString(strings.TrimSpace(m[1]))
	}
	if m := descRe.FindStringSubmatch(detail); m != nil {
		text := html.UnescapeString(strings.TrimSpace(tagStripRe.ReplaceAllString(m[1], "")))
		scene.Description = strings.Join(strings.Fields(text), " ")
	}
	if m := posterRe.FindStringSubmatch(detail); m != nil {
		scene.Thumbnail = normalizeURL(m[1])
	}

	return scene, false, nil
}

// parseDuration converts "31Min 19sec" to seconds.
func parseDuration(detail string) int {
	m := durationRe.FindStringSubmatch(detail)
	if m == nil {
		return 0
	}
	return atoi(m[1])*60 + atoi(m[2])
}

func dedupe(re *regexp.Regexp, detail string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, m := range re.FindAllStringSubmatch(detail, -1) {
		v := html.UnescapeString(strings.TrimSpace(m[1]))
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// normalizeURL upgrades the CDN's protocol-relative URLs to https.
func normalizeURL(u string) string {
	if strings.HasPrefix(u, "//") {
		return "https:" + u
	}
	return u
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return n
		}
		n = n*10 + int(r-'0')
	}
	return n
}
