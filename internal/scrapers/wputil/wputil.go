package wputil

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

// Meta holds metadata extracted from a WordPress page's HTML.
type Meta struct {
	Title       string
	Date        time.Time
	Description string
	Thumbnail   string
	PostID      string
	Tags        []string // from <meta property="article:tag">
	Categories  []string // from articleSection in JSON-LD
	Width       int      // from VideoObject JSON-LD
	Height      int      // from VideoObject JSON-LD
	HasVideo    bool     // true if a VideoObject JSON-LD block was found
}

// BrowserHeaders returns common browser headers to avoid WAF blocks.
func BrowserHeaders() map[string]string {
	return map[string]string{
		"User-Agent":      httpx.UserAgentFirefox,
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language": "en-US,en;q=0.5",
	}
}

// ---- sitemap ----

type urlset struct {
	URLs []SitemapURL `xml:"url"`
}

type SitemapURL struct {
	Loc string `xml:"loc"`
}

func FetchSitemap(ctx context.Context, client *http.Client, sitemapURL string, headers map[string]string) ([]SitemapURL, error) {
	resp, err := httpx.Do(ctx, client, httpx.Request{
		URL:     sitemapURL,
		Headers: headers,
	})
	if err != nil {
		return nil, fmt.Errorf("fetching sitemap %s: %w", sitemapURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading sitemap: %w", err)
	}

	var us urlset
	if err := xml.Unmarshal(body, &us); err != nil {
		return nil, fmt.Errorf("parsing sitemap XML: %w", err)
	}
	return us.URLs, nil
}

// FetchAllSitemaps fetches multiple sitemaps and returns the combined URL list.
func FetchAllSitemaps(ctx context.Context, client *http.Client, urls []string, headers map[string]string) ([]SitemapURL, error) {
	var all []SitemapURL
	for _, u := range urls {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		entries, err := FetchSitemap(ctx, client, u, headers)
		if err != nil {
			return nil, err
		}
		all = append(all, entries...)
	}
	return all, nil
}

// ---- page fetching ----

func FetchPage(ctx context.Context, client *http.Client, pageURL string, headers map[string]string) ([]byte, error) {
	resp, err := httpx.Do(ctx, client, httpx.Request{
		URL:     pageURL,
		Headers: headers,
	})
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", pageURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", pageURL, err)
	}
	return body, nil
}

// ---- metadata extraction ----

var (
	titleRe          = regexp.MustCompile(`<title>([^<]+)</title>`)
	publishedRe      = regexp.MustCompile(`article:published_time"\s+content="([^"]+)"`)
	ogDescRe         = regexp.MustCompile(`og:description"\s+content="([^"]*)"`)
	ogImageRe        = regexp.MustCompile(`og:image"\s+content="([^"]+)"`)
	shortlinkRe      = regexp.MustCompile(`rel='shortlink'\s+href='[^?]*\?p=(\d+)'`)
	articleTagRe     = regexp.MustCompile(`article:tag"\s+content="([^"]*)"`)
	articleSectionRe = regexp.MustCompile(`"articleSection"\s*:\s*"([^"]*)"`)
	videoWidthRe     = regexp.MustCompile(`"@type"\s*:\s*"VideoObject"[^}]*"width"\s*:\s*"(\d+)"`)
	videoHeightRe    = regexp.MustCompile(`"@type"\s*:\s*"VideoObject"[^}]*"height"\s*:\s*"(\d+)"`)
	hasVideoRe       = regexp.MustCompile(`"@type"\s*:\s*"VideoObject"`)
	ldDescRe         = regexp.MustCompile(`"@type"\s*:\s*"VideoObject"[^}]*"description"\s*:\s*"([^"]*)"`)
	ldThumbnailRe    = regexp.MustCompile(`"@type"\s*:\s*"VideoObject"[^}]*"thumbnailUrl"\s*:\s*"([^"]*)"`)
	ldUploadDateRe   = regexp.MustCompile(`"@type"\s*:\s*"VideoObject"[^}]*"uploadDate"\s*:\s*"([^"]*)"`)
)

// ParseMeta extracts common WordPress metadata from raw HTML.
// titleSuffix is stripped from the <title> tag (e.g. " - Mom Comes First").
func ParseMeta(body []byte, titleSuffix string) Meta {
	var m Meta

	if match := titleRe.FindSubmatch(body); match != nil {
		m.Title = html.UnescapeString(string(match[1]))
		m.Title = strings.TrimSuffix(m.Title, titleSuffix)
		m.Title = strings.TrimSpace(m.Title)
	}

	if match := publishedRe.FindSubmatch(body); match != nil {
		if t, err := time.Parse(time.RFC3339, string(match[1])); err == nil {
			m.Date = t.UTC()
		}
	}

	if match := ogDescRe.FindSubmatch(body); match != nil {
		m.Description = html.UnescapeString(string(match[1]))
	}

	if match := ogImageRe.FindSubmatch(body); match != nil {
		m.Thumbnail = string(match[1])
	}

	if match := shortlinkRe.FindSubmatch(body); match != nil {
		m.PostID = string(match[1])
	}

	seen := make(map[string]bool)
	for _, match := range articleTagRe.FindAllSubmatch(body, -1) {
		tag := html.UnescapeString(strings.TrimSpace(string(match[1])))
		if tag != "" && !seen[tag] {
			seen[tag] = true
			m.Tags = append(m.Tags, tag)
		}
	}

	if match := articleSectionRe.FindSubmatch(body); match != nil {
		for _, cat := range strings.Split(string(match[1]), ",") {
			cat = html.UnescapeString(strings.TrimSpace(cat))
			if cat != "" {
				m.Categories = append(m.Categories, cat)
			}
		}
	}

	if match := videoWidthRe.FindSubmatch(body); match != nil {
		m.Width, _ = strconv.Atoi(string(match[1]))
	}
	if match := videoHeightRe.FindSubmatch(body); match != nil {
		m.Height, _ = strconv.Atoi(string(match[1]))
	}

	m.HasVideo = hasVideoRe.Match(body)

	// JSON-LD VideoObject fallbacks for sites without og: meta tags.
	if m.Description == "" {
		if match := ldDescRe.FindSubmatch(body); match != nil {
			m.Description = html.UnescapeString(string(match[1]))
		}
	}
	if m.Thumbnail == "" {
		if match := ldThumbnailRe.FindSubmatch(body); match != nil {
			m.Thumbnail = string(match[1])
		}
	}
	if m.Date.IsZero() {
		if match := ldUploadDateRe.FindSubmatch(body); match != nil {
			if t, err := time.Parse(time.RFC3339, string(match[1])); err == nil {
				m.Date = t.UTC()
			}
		}
	}

	return m
}

// SlugFromURL extracts the last path segment, stripping .html extension.
// Trailing slashes are removed before .html stripping so inputs like
// "https://x/post.html/" still resolve to "post".
func SlugFromURL(pageURL string) string {
	pageURL = strings.TrimRight(pageURL, "/")
	pageURL = strings.TrimSuffix(pageURL, ".html")
	parts := strings.Split(pageURL, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return pageURL
}

// ParseDuration converts "MM:SS" or "H:MM:SS" to seconds.
func ParseDuration(s string) int {
	parts := strings.Split(s, ":")
	total := 0
	for _, p := range parts {
		n, _ := strconv.Atoi(p)
		total = total*60 + n
	}
	return total
}

// VideoWidth returns the standard width for a given height.
func VideoWidth(height int) int {
	switch {
	case height >= 2160:
		return 3840
	case height >= 1080:
		return 1920
	case height >= 720:
		return 1280
	case height >= 480:
		return 854
	default:
		return 0
	}
}

// ---- worker pool ----

// PageParser is called for each page. Returns a scene, whether to skip, and any error.
type PageParser func(studioURL, pageURL string, body []byte, now time.Time) (models.Scene, bool, error)

// RunWorkerPool fetches sitemaps, then dispatches pages to workers that call parse.
func RunWorkerPool(ctx context.Context, client *http.Client, headers map[string]string,
	sitemapURLs []string, studioURL string, opts scraper.ListOpts,
	parse PageParser, out chan<- scraper.SceneResult) {

	allURLs, err := FetchAllSitemaps(ctx, client, sitemapURLs, headers)
	if err != nil {
		select {
		case out <- scraper.Error(err):
		case <-ctx.Done():
		}
		return
	}

	if len(allURLs) > 0 {
		select {
		case out <- scraper.Progress(len(allURLs)):
		case <-ctx.Done():
			return
		}
	}

	if opts.Workers <= 0 {
		opts.Workers = 3
	}

	work := make(chan SitemapURL, opts.Workers)
	var wg sync.WaitGroup

	for i := 0; i < opts.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for entry := range work {
				if ctx.Err() != nil {
					return
				}
				if opts.Delay > 0 {
					select {
					case <-time.After(opts.Delay):
					case <-ctx.Done():
						return
					}
				}

				body, fetchErr := FetchPage(ctx, client, entry.Loc, headers)
				if fetchErr != nil {
					select {
					case out <- scraper.Error(fetchErr):
					case <-ctx.Done():
						return
					}
					continue
				}

				scene, skip, parseErr := parse(studioURL, entry.Loc, body, time.Now().UTC())
				if parseErr != nil {
					select {
					case out <- scraper.Error(parseErr):
					case <-ctx.Done():
						return
					}
					continue
				}
				if skip {
					continue
				}
				if len(opts.KnownIDs) > 0 && opts.KnownIDs[scene.ID] {
					continue
				}
				select {
				case out <- scraper.Scene(scene):
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	for _, u := range allURLs {
		select {
		case work <- u:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
	}

	close(work)
	wg.Wait()
}
