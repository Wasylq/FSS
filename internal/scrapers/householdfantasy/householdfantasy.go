// Package householdfantasy scrapes Household Fantasy (householdfantasy.com), a
// WordPress site whose scenes are ordinary posts.
//
// The WP REST API is blocked (HTTP 403 from the host's WAF), so enumeration
// goes through the Yoast post sitemap instead — ~344 posts, one per scene.
//
// Two things differ from the usual wputil site and are handled here:
//
//   - There is no VideoObject JSON-LD, so wputil's HasVideo flag is always
//     false and cannot be used to tell scenes from non-scene posts. The post
//     sitemap holds only scenes, so every entry is kept.
//   - Performers are not in <meta property="article:tag"> (the site emits
//     none). They are rendered as /tag/{slug}/ anchors in the post body, which
//     is where this package reads them from. The JSON-LD articleSection list
//     carries the genre categories separately.
package householdfantasy

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
	siteID     = "householdfantasy"
	studioName = "Household Fantasy"
)

var siteBase = "https://householdfantasy.com"

// Scraper implements scraper.StudioScraper for Household Fantasy.
type Scraper struct {
	client  *http.Client
	headers map[string]string
}

// New constructs a Household Fantasy scraper.
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
		"householdfantasy.com",
		"householdfantasy.com/{slug}/",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?householdfantasy\.com(?:/|$)`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	if opts.Workers <= 0 {
		opts.Workers = 3
	}
	out := make(chan scraper.SceneResult)
	go func() {
		defer close(out)
		scraper.Debugf(1, "%s: enumerating post sitemap", siteID)
		wputil.RunWorkerPool(ctx, s.client, s.headers,
			[]string{siteBase + "/post-sitemap.xml"},
			studioURL, opts, parsePage, out)
	}()
	return out, nil
}

var (
	titleSuffixRe = regexp.MustCompile(`\s*-\s*Household Fantasy$`)
	// Performers live in the JSON-LD "keywords" array. The site emits no
	// article:tag meta, so wputil's Tags is always empty here.
	keywordsRe = regexp.MustCompile(`"keywords"\s*:\s*\[([^\]]*)\]`)
	// Theme-level fallback for performers: the same names are rendered as
	// /tag/ anchors in the post body.
	performerRe = regexp.MustCompile(`<a[^>]+href="[^"]*/tag/[a-z0-9-]+/"[^>]*class="[^"]*elementor-post-info__terms-list-item[^"]*"[^>]*>([^<]+)</a>`)
	// Genre categories. wputil only understands the string form
	// ("articleSection":"X"); this site emits a JSON array, so it is parsed
	// here rather than by changing shared behaviour for other WP sites.
	articleSectionRe = regexp.MustCompile(`"articleSection"\s*:\s*\[([^\]]*)\]`)
	// postid-{n} on <body> is the only stable numeric id on the page; the
	// wp shortlink meta tag is absent.
	postIDRe = regexp.MustCompile(`\bpostid-(\d+)\b`)
)

func parsePage(studioURL, pageURL string, body []byte, now time.Time) (models.Scene, bool, error) {
	meta := wputil.ParseMeta(body, "")
	meta.Title = titleSuffixRe.ReplaceAllString(meta.Title, "")

	// Prefer the numeric WordPress post id; fall back to the slug so a scene
	// still gets a stable id if the body class ever changes.
	id := ""
	if m := postIDRe.FindSubmatch(body); m != nil {
		id = string(m[1])
	}
	if id == "" {
		id = wputil.SlugFromURL(pageURL)
	}

	categories := meta.Categories
	if len(categories) == 0 {
		categories = parseJSONArray(articleSectionRe, body)
	}

	scene := models.Scene{
		ID:          id,
		SiteID:      siteID,
		StudioURL:   studioURL,
		Title:       meta.Title,
		URL:         pageURL,
		Date:        meta.Date,
		Description: meta.Description,
		Thumbnail:   meta.Thumbnail,
		Studio:      studioName,
		Categories:  categories,
		Performers:  parsePerformers(body),
		ScrapedAt:   now,
	}
	return scene, false, nil
}

// parsePerformers reads the JSON-LD keywords array, falling back to the
// theme's /tag/ anchors if it is absent.
func parsePerformers(body []byte) []string {
	if names := parseJSONArray(keywordsRe, body); len(names) > 0 {
		return names
	}

	var out []string
	seen := make(map[string]bool)
	for _, m := range performerRe.FindAllSubmatch(body, -1) {
		name := html.UnescapeString(strings.TrimSpace(string(m[1])))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

// parseJSONArray pulls the quoted strings out of a JSON string-array captured
// by re's first group, e.g. `"A","B"` -> ["A", "B"].
func parseJSONArray(re *regexp.Regexp, body []byte) []string {
	m := re.FindSubmatch(body)
	if m == nil {
		return nil
	}
	var out []string
	seen := make(map[string]bool)
	for _, q := range quotedRe.FindAllSubmatch(m[1], -1) {
		v := html.UnescapeString(strings.TrimSpace(string(q[1])))
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

var quotedRe = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"`)
