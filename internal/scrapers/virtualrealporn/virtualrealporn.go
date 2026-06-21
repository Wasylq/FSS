// Package virtualrealporn scrapes the VirtualRealPorn VR network — a family of
// WordPress sites that share one CMS: VirtualRealPorn, VirtualRealGay,
// VirtualRealTrans, VirtualRealJapan and VirtualRealPassion. It is a
// table-driven package: one scraper is registered per site in init().
//
// The public scene catalogue is exposed through the Yoast "pelicula" (movie)
// XML sitemaps, which list every scene URL in publish order (oldest first).
// The scraper walks the sitemaps newest-first (reversed) and enriches each
// scene from its detail page, which carries a schema.org Movie JSON-LD block
// with title, description, duration, publish date, performers and keywords.
//
// VirtualRealAmateur (virtualrealamateurporn.com) is intentionally not covered:
// the domain 301-redirects to virtualrealporn.com and no longer publishes its
// own catalogue.
package virtualrealporn

import (
	"context"
	"encoding/json"
	"encoding/xml"
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

// pageSize is how many scene detail pages are fetched per pagination chunk.
// It mirrors the network's own listing page size (24) so progress and the
// KnownIDs early-stop behave like a normal page-numbered scraper.
const pageSize = 24

// SiteConfig describes one VirtualReal network site served by this package.
type SiteConfig struct {
	SiteID     string // stable lowercase id, e.g. "virtualrealporn"
	Domain     string // bare domain, e.g. "virtualrealporn.com"
	StudioName string // display name, e.g. "VirtualRealPorn"
}

var sites = []SiteConfig{
	{SiteID: "virtualrealporn", Domain: "virtualrealporn.com", StudioName: "VirtualRealPorn"},
	{SiteID: "virtualrealgay", Domain: "virtualrealgay.com", StudioName: "VirtualRealGay"},
	{SiteID: "virtualrealtrans", Domain: "virtualrealtrans.com", StudioName: "VirtualRealTrans"},
	{SiteID: "virtualrealjapan", Domain: "virtualrealjapan.com", StudioName: "VirtualRealJapan"},
	{SiteID: "virtualrealpassion", Domain: "virtualrealpassion.com", StudioName: "VirtualRealPassion"},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(newFor(cfg.SiteID))
	}
}

// newFor builds the registered scraper for a given site id. It is also used by
// the integration tests.
func newFor(siteID string) *Scraper {
	for _, cfg := range sites {
		if cfg.SiteID == siteID {
			return New(cfg)
		}
	}
	return nil
}

// Scraper implements scraper.StudioScraper for a single VirtualReal site.
type Scraper struct {
	cfg     SiteConfig
	Client  *http.Client
	base    string
	matchRe *regexp.Regexp
}

var _ scraper.StudioScraper = (*Scraper)(nil)

// New constructs a Scraper for the given site config.
func New(cfg SiteConfig) *Scraper {
	escaped := regexp.QuoteMeta(cfg.Domain)
	return &Scraper{
		cfg:     cfg,
		Client:  httpx.NewClient(30 * time.Second),
		base:    "https://" + cfg.Domain,
		matchRe: regexp.MustCompile(`^https?://(?:www\.)?` + escaped + `(?:/|$)`),
	}
}

func (s *Scraper) ID() string { return s.cfg.SiteID }

func (s *Scraper) Patterns() []string {
	return []string{
		s.cfg.Domain,
		s.cfg.Domain + "/latest-videos/",
	}
}

func (s *Scraper) MatchesURL(u string) bool { return s.matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, opts, out)
	return out, nil
}

// sitemapURLSet maps the Yoast sitemap <urlset> for a single sitemap file.
type sitemapURLSet struct {
	URLs []struct {
		Loc string `xml:"loc"`
	} `xml:"url"`
}

// sitemapIndex maps the Yoast sitemap index, used to discover every paginated
// pelicula sitemap file (pelicula-sitemap.xml, pelicula-sitemap2.xml, …).
type sitemapIndex struct {
	Sitemaps []struct {
		Loc string `xml:"loc"`
	} `xml:"sitemap"`
}

func (s *Scraper) run(ctx context.Context, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	scraper.Debugf(1, "%s: discovering scene sitemaps", s.cfg.SiteID)
	urls, err := s.collectSceneURLs(ctx)
	if err != nil {
		select {
		case out <- scraper.Error(err):
		case <-ctx.Done():
		}
		return
	}
	if len(urls) == 0 {
		return
	}
	scraper.Debugf(1, "%s: %d scenes in catalogue", s.cfg.SiteID, len(urls))

	now := time.Now().UTC()
	totalPages := (len(urls) + pageSize - 1) / pageSize

	scraper.Paginate(ctx, opts, s.cfg.SiteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		start := (page - 1) * pageSize
		if start >= len(urls) {
			return scraper.PageResult{}, nil
		}
		end := start + pageSize
		if end > len(urls) {
			end = len(urls)
		}
		chunk := urls[start:end]

		total := 0
		if page == 1 {
			total = len(urls)
		}

		scenes := s.fetchDetails(ctx, chunk, opts, now)
		return scraper.PageResult{
			Scenes: scenes,
			Total:  total,
			Done:   page >= totalPages,
		}, nil
	})
}

// collectSceneURLs returns every scene detail URL for the site, ordered
// newest-first. The Yoast pelicula sitemaps list scenes oldest-first across one
// or more files (pelicula-sitemap.xml, pelicula-sitemap2.xml, …); both the file
// order and the within-file order are reversed so the freshest scene comes
// first, which is what KnownIDs early-stop relies on.
func (s *Scraper) collectSceneURLs(ctx context.Context) ([]string, error) {
	files, err := s.sceneSitemapFiles(ctx)
	if err != nil {
		return nil, err
	}

	var urls []string
	// Files are reversed: the highest-numbered sitemap holds the newest scenes.
	for i := len(files) - 1; i >= 0; i-- {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		body, err := s.fetchPage(ctx, files[i])
		if err != nil {
			return nil, err
		}
		var set sitemapURLSet
		if err := xml.Unmarshal(body, &set); err != nil {
			return nil, fmt.Errorf("parse sitemap %s: %w", files[i], err)
		}
		// Reverse within the file so the newest scene leads.
		for j := len(set.URLs) - 1; j >= 0; j-- {
			loc := strings.TrimSpace(set.URLs[j].Loc)
			if loc != "" {
				urls = append(urls, loc)
			}
		}
	}
	return urls, nil
}

// sceneSitemapFiles discovers the pelicula sitemap file URLs from the sitemap
// index. The files are returned in index order (oldest first).
func (s *Scraper) sceneSitemapFiles(ctx context.Context) ([]string, error) {
	body, err := s.fetchPage(ctx, s.base+"/sitemap_index.xml")
	if err != nil {
		return nil, err
	}
	var idx sitemapIndex
	if err := xml.Unmarshal(body, &idx); err != nil {
		return nil, fmt.Errorf("parse sitemap index: %w", err)
	}
	var files []string
	for _, sm := range idx.Sitemaps {
		loc := strings.TrimSpace(sm.Loc)
		if strings.Contains(loc, "pelicula-sitemap") {
			files = append(files, loc)
		}
	}
	return files, nil
}

// fetchDetails enriches each scene URL from its detail page with a worker pool.
// Order is preserved so Paginate's KnownIDs early-stop fires on the right
// scene; known IDs become lightweight stubs (no detail fetch) and detail-fetch
// failures are dropped.
func (s *Scraper) fetchDetails(ctx context.Context, urls []string, opts scraper.ListOpts, now time.Time) []models.Scene {
	workers := opts.Workers
	if workers <= 0 {
		workers = 4
	}
	scraper.Debugf(1, "%s: fetching %d details with %d workers", s.cfg.SiteID, len(urls), workers)

	results := make([]models.Scene, len(urls))
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)

	for i, u := range urls {
		if ctx.Err() != nil {
			break
		}
		id := sceneID(u)
		if opts.KnownIDs[id] {
			results[i] = models.Scene{ID: id, SiteID: s.cfg.SiteID}
			continue
		}
		wg.Add(1)
		go func(idx int, rawURL, id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if opts.Delay > 0 {
				select {
				case <-time.After(opts.Delay):
				case <-ctx.Done():
					return
				}
			}

			body, err := s.fetchPage(ctx, rawURL)
			if err != nil {
				scraper.Debugf(1, "%s: detail %s failed: %v (skipping)", s.cfg.SiteID, id, err)
				return
			}
			if sc, ok := s.toScene(body, rawURL, id, now); ok {
				results[idx] = sc
			}
		}(i, u, id)
	}
	wg.Wait()

	scenes := make([]models.Scene, 0, len(results))
	for _, sc := range results {
		if sc.ID == "" { // failed fetch
			continue
		}
		scenes = append(scenes, sc)
	}
	return scenes
}

// ldMovie is the subset of the schema.org Movie JSON-LD block on a detail page.
type ldMovie struct {
	Type          string          `json:"@type"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Image         json.RawMessage `json:"image"`
	Duration      string          `json:"duration"`
	DatePublished string          `json:"datePublished"`
	Keywords      string          `json:"keywords"`
	Genre         string          `json:"genre"`
	Actors        []struct {
		Name string `json:"name"`
	} `json:"actors"`
}

var (
	ldJSONRe = regexp.MustCompile(`(?s)<script type="application/ld\+json"[^>]*>(.*?)</script>`)
	slugRe   = regexp.MustCompile(`/([a-z0-9-]+)/?$`)
)

func (s *Scraper) toScene(body []byte, rawURL, id string, now time.Time) (models.Scene, bool) {
	movie, ok := extractMovie(body)
	if !ok {
		return models.Scene{}, false
	}

	scene := models.Scene{
		ID:          id,
		SiteID:      s.cfg.SiteID,
		StudioURL:   s.base,
		Title:       cleanTitle(movie.Name),
		URL:         rawURL,
		Studio:      s.cfg.StudioName,
		Description: cleanText(movie.Description),
		Thumbnail:   firstImage(movie.Image),
		Duration:    parseutil.ParseDurationColon(movie.Duration),
		Performers:  actorNames(movie.Actors),
		Tags:        sceneTags(movie.Keywords, movie.Genre),
		ScrapedAt:   now,
	}
	if t, err := parseutil.TryParseDate(strings.TrimSpace(movie.DatePublished),
		time.RFC3339, "2006-01-02T15:04:05Z07:00", "2006-01-02"); err == nil {
		scene.Date = t.UTC()
	}
	if scene.Title == "" {
		scene.Title = titleFromSlug(id)
	}
	return scene, true
}

// extractMovie parses the first schema.org Movie (or VideoObject) JSON-LD block.
func extractMovie(body []byte) (ldMovie, bool) {
	for _, m := range ldJSONRe.FindAllSubmatch(body, -1) {
		blk := strings.TrimSpace(string(m[1]))
		if !strings.Contains(blk, `"Movie"`) && !strings.Contains(blk, `"VideoObject"`) {
			continue
		}
		var mv ldMovie
		if err := json.Unmarshal([]byte(blk), &mv); err != nil {
			continue
		}
		if mv.Type == "Movie" || mv.Type == "VideoObject" {
			return mv, true
		}
	}
	return ldMovie{}, false
}

// sceneID returns the trailing slug of a scene URL, used as the stable scene ID.
func sceneID(rawURL string) string {
	if m := slugRe.FindStringSubmatch(rawURL); m != nil {
		return m[1]
	}
	return strings.TrimRight(rawURL, "/")
}

// cleanTitle strips the site suffix (e.g. " | VirtualRealPorn VR Porn video")
// that the CMS appends to every JSON-LD name.
func cleanTitle(name string) string {
	name = cleanText(name)
	if i := strings.Index(name, " | "); i >= 0 {
		name = name[:i]
	}
	return strings.TrimSpace(name)
}

// titleFromSlug builds a readable fallback title from the URL slug.
func titleFromSlug(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func actorNames(actors []struct {
	Name string `json:"name"`
}) []string {
	if len(actors) == 0 {
		return nil
	}
	var out []string
	for _, a := range actors {
		if n := cleanText(a.Name); n != "" {
			out = append(out, n)
		}
	}
	return out
}

// sceneTags splits the JSON-LD keywords (comma-separated), dropping the generic
// "genre" prefix that every scene shares (vr porn, virtual reality, resolutions).
func sceneTags(keywords, genre string) []string {
	generic := make(map[string]bool)
	for _, g := range strings.Split(genre, ",") {
		generic[strings.ToLower(strings.TrimSpace(g))] = true
	}
	seen := make(map[string]bool)
	var out []string
	for _, k := range strings.Split(keywords, ",") {
		tag := cleanText(k)
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if generic[key] || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, tag)
	}
	return out
}

// firstImage returns the first image URL from the JSON-LD "image" field, which
// may be a bare string or an array of strings.
func firstImage(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return cleanText(single)
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return cleanText(arr[0])
	}
	return ""
}

func (s *Scraper) fetchPage(ctx context.Context, url string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL:     url,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}

func cleanText(s string) string {
	return html.UnescapeString(strings.TrimSpace(s))
}
