// Package bananafever scrapes bananafever.com, a Next.js App Router site.
//
// There is no `__NEXT_DATA__` blob to lift — the App Router ships RSC flight
// data instead — but every scene page carries a clean schema.org `VideoObject`
// with the title, description, thumbnail, upload date, cast and stream URL, so
// `parseutil.ExtractVideoObject` does the parsing.
//
// Enumeration runs off `/sitemap-videos.xml`, which the sitemap index names.
// The `/videos?page={N}` listing works too but is 49 pages against one request,
// and the sitemap is the only one of the two that is a complete list.
//
// The site is multilingual: every sitemap entry advertises `/cn/video/…` and
// other `hreflang` alternates for the same scene. Only the `<loc>` is followed,
// so a scene is stored once rather than once per language.
package bananafever

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Anastylosis/FSS/internal/httpx"
	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/parseutil"
	"github.com/Anastylosis/FSS/scraper"
)

const (
	siteID     = "bananafever"
	domain     = "bananafever.com"
	studioName = "BananaFever"
	// videoSitemapPath is named in /sitemap.xml's index, alongside static,
	// talent, category and tag sitemaps that carry no scenes.
	videoSitemapPath = "/sitemap-videos.xml"
)

type Scraper struct {
	Client       *http.Client
	baseOverride string
}

func New() *Scraper {
	return &Scraper{Client: httpx.NewClient(30 * time.Second)}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

var (
	matchRe = regexp.MustCompile(`^https?://(?:www\.)?bananafever\.com(?:/|$)`)
	// Scene paths are `/video/{slug}-{id}`. The `/cn/video/…` and other
	// language prefixes address the same scene and are deliberately not
	// matched — following them would store every scene once per language.
	scenePathRe = regexp.MustCompile(`^/video/([A-Za-z0-9-]+)$`)
	// The trailing token of the slug is the site's own short id.
	sceneIDRe = regexp.MustCompile(`-([A-Za-z0-9]{6})$`)
	locRe     = regexp.MustCompile(`<loc>\s*([^<\s]+)\s*</loc>`)
)

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		domain,
		domain + "/videos",
		domain + "/video/{slug}",
	}
}

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	if opts.Workers <= 0 {
		opts.Workers = 4
	}
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// base resolves the origin to fetch from. The operator's own host wins so a
// non-www spelling stays addressable; baseOverride is the test server. Sitemap
// entries are absolute live URLs, so they are re-based onto this rather than
// fetched verbatim — otherwise an offline test would still hit production.
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

	// A single scene URL is a legitimate thing to point at.
	if p, ok := scenePath(studioURL); ok {
		scraper.Debugf(1, "%s: scraping one scene %s", siteID, p)
		if !send(ctx, out, scraper.Progress(1)) {
			return
		}
		if sc := s.buildScene(ctx, studioURL, base, p, out); sc.ID != "" {
			send(ctx, out, scraper.Scene(sc))
		}
		return
	}

	scraper.Debugf(1, "%s: reading the video sitemap", siteID)
	body, err := s.fetch(ctx, base+videoSitemapPath)
	if err != nil {
		send(ctx, out, scraper.Error(fmt.Errorf("video sitemap: %w", err)))
		return
	}

	seen := make(map[string]bool)
	var paths []string
	for _, m := range locRe.FindAllStringSubmatch(string(body), -1) {
		p, ok := scenePath(html.UnescapeString(m[1]))
		if !ok || seen[p] {
			continue
		}
		seen[p] = true
		paths = append(paths, p)
	}
	if len(paths) == 0 {
		// A sitemap that fetched cleanly and named no scenes is a feed change,
		// not an empty catalogue, and must not read as one to an authoritative
		// --full save.
		send(ctx, out, scraper.Error(scraper.ParseError(base+videoSitemapPath,
			fmt.Errorf("no scene URLs in the video sitemap"))))
		return
	}

	scraper.Debugf(1, "%s: %d scenes in the sitemap, fetching with %d workers", siteID, len(paths), opts.Workers)
	if !send(ctx, out, scraper.Progress(len(paths))) {
		return
	}
	s.fetchAll(ctx, studioURL, base, paths, opts, out)
}

// scenePath reports whether a URL names a scene page, and returns its path.
func scenePath(rawURL string) (string, bool) {
	p := rawURL
	if u, err := url.Parse(rawURL); err == nil && u.Path != "" {
		p = u.Path
	}
	p = strings.TrimSuffix(strings.TrimSpace(p), "/")
	if !scenePathRe.MatchString(p) {
		return "", false
	}
	return p, true
}

func (s *Scraper) fetchAll(ctx context.Context, studioURL, base string, paths []string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	work := make(chan string)
	var wg sync.WaitGroup
	// LIFO: close(work) ends the workers' range loops, then wg.Wait blocks
	// until they are gone, so a ctx.Done bail below cannot leak them.
	defer wg.Wait()
	defer close(work)

	for i := 0; i < opts.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range work {
				if !sleep(ctx, opts.Delay) {
					return
				}
				sc := s.buildScene(ctx, studioURL, base, p, out)
				if sc.ID == "" {
					continue
				}
				if !send(ctx, out, scraper.Scene(sc)) {
					return
				}
			}
		}()
	}

	for _, p := range paths {
		select {
		case work <- p:
		case <-ctx.Done():
			return
		}
	}
}

// buildScene fetches one scene page. A zero-value ID means it could not be read
// and the failure has already been reported.
func (s *Scraper) buildScene(ctx context.Context, studioURL, base, path string, out chan<- scraper.SceneResult) models.Scene {
	sceneURL := base + path

	body, err := s.fetch(ctx, sceneURL)
	if err != nil {
		send(ctx, out, scraper.Error(fmt.Errorf("scene %s: %w", path, err)))
		return models.Scene{}
	}

	vo := parseutil.ExtractVideoObject(body)
	if vo == nil || strings.TrimSpace(vo.Name) == "" {
		send(ctx, out, scraper.Error(scraper.ParseError(sceneURL,
			fmt.Errorf("no VideoObject on the scene page"))))
		return models.Scene{}
	}

	id := firstSubmatch(sceneIDRe, path)
	if id == "" {
		// No short id on the tail — the slug is the next-best stable key.
		id = strings.TrimPrefix(path, "/video/")
	}

	scene := models.Scene{
		ID:          id,
		SiteID:      siteID,
		StudioURL:   studioURL,
		Title:       cleanText(vo.Name),
		URL:         sceneURL,
		Studio:      studioName,
		Description: cleanText(vo.Description),
		Thumbnail:   vo.ThumbnailURL,
		Performers:  cleanNames(vo.Actors),
		ScrapedAt:   time.Now().UTC(),
	}
	if d := parseutil.ParseDurationISO(vo.Duration); d > 0 {
		scene.Duration = d
	}
	for _, v := range []string{vo.UploadDate, vo.DatePublished} {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(v)); err == nil {
			scene.Date = t.UTC()
			break
		}
	}
	if kw := cleanText(vo.Keywords); kw != "" {
		scene.Tags = splitKeywords(kw)
	}
	return scene
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

// ---- helpers ----

var tagStripRe = regexp.MustCompile(`<[^>]*>`)

func cleanText(s string) string {
	s = tagStripRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, " ", " ")
	return strings.Join(strings.Fields(s), " ")
}

func cleanNames(in []string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, v := range in {
		n := cleanText(v)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}

func splitKeywords(s string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, part := range strings.Split(s, ",") {
		t := strings.TrimSpace(part)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

func firstSubmatch(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	select {
	case <-time.After(d):
		return true
	case <-ctx.Done():
		return false
	}
}

func send(ctx context.Context, out chan<- scraper.SceneResult, r scraper.SceneResult) bool {
	select {
	case out <- r:
		return true
	case <-ctx.Done():
		return false
	}
}
