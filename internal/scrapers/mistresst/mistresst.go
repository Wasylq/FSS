// Package mistresst scrapes Mistress T (https://www.mistresst.net/), a Drupal
// femdom clip site. Enumeration is sitemap-based: sitemap.xml lists ~2.5k scene
// URLs of the form /content/{slug}. Each detail page is a Drupal node carrying
// og: metadata plus field-* blocks for the post date, category, tags and poster.
// Members buy clips on clips4sale, so there is no price on the tour pages.
package mistresst

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/parseutil"
	"github.com/Wasylq/FSS/scraper"
)

const (
	siteID        = "mistresst"
	studioName    = "Mistress T"
	siteBase      = "https://www.mistresst.net"
	sitemapURL    = siteBase + "/sitemap.xml"
	defaultWorker = 6
)

// Scraper implements scraper.StudioScraper for Mistress T.
type Scraper struct {
	Client *http.Client
}

// New constructs a Mistress T scraper.
func New() *Scraper {
	return &Scraper{Client: httpx.NewClient(30 * time.Second)}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		"mistresst.net",
		"mistresst.net/content/{slug}",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?mistresst\.net`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// ---- runner ----

var locRe = regexp.MustCompile(`<loc>\s*([^<]+?)\s*</loc>`)

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()

	scraper.Debugf(1, "%s: fetching sitemap %s", siteID, sitemapURL)
	body, err := s.get(ctx, sitemapURL)
	if err != nil {
		select {
		case out <- scraper.Error(fmt.Errorf("sitemap: %w", err)):
		case <-ctx.Done():
		}
		return
	}

	urls := sceneURLs(body)
	scraper.Debugf(1, "%s: %d scene URLs in sitemap", siteID, len(urls))

	select {
	case out <- scraper.Progress(len(urls)):
	case <-ctx.Done():
		return
	}

	workers := defaultWorker
	if opts.Workers > 0 {
		workers = opts.Workers
	}
	scraper.Debugf(1, "%s: fetching %d details with %d workers", siteID, len(urls), workers)

	jobs := make(chan string)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for u := range jobs {
				if ctx.Err() != nil {
					return
				}
				scene, err := s.fetchScene(ctx, studioURL, u, now)
				if err != nil {
					select {
					case out <- scraper.Error(fmt.Errorf("%s: %w", u, err)):
					case <-ctx.Done():
						return
					}
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

	for _, u := range urls {
		select {
		case jobs <- u:
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return
		}
	}
	close(jobs)
	wg.Wait()
}

// sceneURLs extracts the /content/{slug} scene URLs from the sitemap, skipping
// taxonomy/static pages and the /content/front homepage.
func sceneURLs(body []byte) []string {
	matches := locRe.FindAllSubmatch(body, -1)
	seen := make(map[string]bool, len(matches))
	urls := make([]string, 0, len(matches))
	for _, m := range matches {
		u := string(m[1])
		if !strings.Contains(u, "/content/") {
			continue
		}
		if strings.HasSuffix(u, "/content/front") {
			continue
		}
		if seen[u] {
			continue
		}
		seen[u] = true
		urls = append(urls, u)
	}
	return urls
}

// ---- detail parsing ----

var (
	titleRe    = regexp.MustCompile(`(?s)<title>\s*(.*?)\s*(?:\|\s*Mistress T)?\s*</title>`)
	postDateRe = regexp.MustCompile(`(?s)field-name-post-date.*?<div class="field-item even">\s*([^<]+?)\s*</div>`)
	categoryRe = regexp.MustCompile(`(?s)field-name-field-video-category.*?<a [^>]*>\s*([^<]+?)\s*</a>`)
	tagsBlock  = regexp.MustCompile(`(?s)field-name-field-tags.*?</div></div></div>`)
	tagsAnchor = regexp.MustCompile(`<a href="/tags/[^"]*">\s*([^<]+?)\s*</a>`)
	posterRe   = regexp.MustCompile(`(?s)field-name-field-video-poster.*?<img [^>]*src="([^"]+)"`)
)

func (s *Scraper) fetchScene(ctx context.Context, studioURL, pageURL string, now time.Time) (models.Scene, error) {
	body, err := s.get(ctx, pageURL)
	if err != nil {
		return models.Scene{}, err
	}
	detail := string(body)
	og := parseutil.OpenGraph(body)

	slug := pageURL
	if i := strings.LastIndex(slug, "/content/"); i >= 0 {
		slug = slug[i+len("/content/"):]
	}

	url := pageURL
	if v := og["og:url"]; v != "" {
		url = html.UnescapeString(v)
	}

	title := html.UnescapeString(og["og:title"])
	if title == "" {
		if m := titleRe.FindStringSubmatch(detail); m != nil {
			title = html.UnescapeString(strings.TrimSpace(m[1]))
		}
	}

	scene := models.Scene{
		ID:          slug,
		SiteID:      siteID,
		StudioURL:   studioURL,
		Title:       title,
		URL:         url,
		Description: html.UnescapeString(og["og:description"]),
		Studio:      studioName,
		ScrapedAt:   now,
	}

	if m := postDateRe.FindStringSubmatch(detail); m != nil {
		if d, err := parseutil.TryParseDate(strings.TrimSpace(m[1]), "Mon, 01/02/2006 - 15:04"); err == nil {
			scene.Date = d
		}
	}

	if m := categoryRe.FindStringSubmatch(detail); m != nil {
		scene.Categories = []string{html.UnescapeString(strings.TrimSpace(m[1]))}
	}

	if block := tagsBlock.FindString(detail); block != "" {
		seen := make(map[string]bool)
		for _, m := range tagsAnchor.FindAllStringSubmatch(block, -1) {
			tag := html.UnescapeString(strings.TrimSpace(m[1]))
			if tag == "" || seen[tag] {
				continue
			}
			seen[tag] = true
			scene.Tags = append(scene.Tags, tag)
		}
	}

	if m := posterRe.FindStringSubmatch(detail); m != nil {
		scene.Thumbnail = html.UnescapeString(strings.TrimSpace(m[1]))
	}

	return scene, nil
}

func (s *Scraper) get(ctx context.Context, u string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{URL: u, Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox)})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}
