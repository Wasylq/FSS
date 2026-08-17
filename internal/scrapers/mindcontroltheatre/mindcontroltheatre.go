// Package mindcontroltheatre scrapes mindcontroltheatre.com, a clip store.
//
// Every page redirects to `/age?r=…` until an `age=yes` cookie is present, so
// the scraper sends it on every request rather than following the gate — one
// header instead of a round trip and a cookie jar.
//
// `sitemap.xml` names all 411 `/movie/{slug}` pages with a `<lastmod>`, and
// each page carries the title, cast, synopsis, thumbnail, trailer and a
// `data` line with the release date, resolution and runtime. Prices are per
// format (4K / HD / DVD); the cheapest is recorded, since that is what the
// scene costs to obtain.
package mindcontroltheatre

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Anastylosis/FSS/internal/httpx"
	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/scraper"
)

const (
	siteID     = "mindcontroltheatre"
	domain     = "mindcontroltheatre.com"
	studioName = "Mind Control Theatre"
	// ageCookie clears the interstitial. The site sets it itself on
	// `/age-yes`; sending it directly saves the redirect on every request.
	ageCookie = "age=yes"
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
	matchRe = regexp.MustCompile(`^https?://(?:www\.)?mindcontroltheatre\.com(?:/|$)`)
	// `/movies/…` is a browse page and `/movie-image/…` an asset; only
	// `/movie/{slug}` is a scene.
	scenePathRe = regexp.MustCompile(`^/movie/([^/?#]+)$`)
	locRe       = regexp.MustCompile(`<loc>\s*([^<\s]+)\s*</loc>`)
)

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		domain,
		domain + "/movies",
		domain + "/movie/{slug}",
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
// entries are absolute live URLs and are re-based onto this, or an offline test
// would still hit production.
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

	// A single movie URL is a legitimate thing to point at.
	if p, ok := scenePath(studioURL); ok {
		scraper.Debugf(1, "%s: scraping one movie %s", siteID, p)
		if !send(ctx, out, scraper.Progress(1)) {
			return
		}
		if sc := s.buildScene(ctx, studioURL, base, p, out); sc.ID != "" {
			send(ctx, out, scraper.Scene(sc))
		}
		return
	}

	scraper.Debugf(1, "%s: reading the sitemap", siteID)
	body, err := s.fetch(ctx, base+"/sitemap.xml")
	if err != nil {
		send(ctx, out, scraper.Error(fmt.Errorf("sitemap: %w", err)))
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
		// A sitemap that fetched cleanly and named no movies is a feed change,
		// not an empty catalogue, and must not read as one to an authoritative
		// --full save.
		send(ctx, out, scraper.Error(scraper.ParseError(base+"/sitemap.xml",
			fmt.Errorf("no movie URLs in the sitemap"))))
		return
	}

	scraper.Debugf(1, "%s: %d movies in the sitemap, fetching with %d workers", siteID, len(paths), opts.Workers)
	if !send(ctx, out, scraper.Progress(len(paths))) {
		return
	}
	s.fetchAll(ctx, studioURL, base, paths, opts, out)
}

// scenePath reports whether a URL names a movie page, and returns its path.
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

// buildScene fetches one movie page. A zero-value ID means it could not be read
// and the failure has already been reported.
func (s *Scraper) buildScene(ctx context.Context, studioURL, base, path string, out chan<- scraper.SceneResult) models.Scene {
	sceneURL := base + path

	body, err := s.fetch(ctx, sceneURL)
	if err != nil {
		send(ctx, out, scraper.Error(fmt.Errorf("movie %s: %w", path, err)))
		return models.Scene{}
	}

	d := parseMovie(string(body))
	if d.title == "" {
		send(ctx, out, scraper.Error(scraper.ParseError(sceneURL, fmt.Errorf("no movie block on the page"))))
		return models.Scene{}
	}

	scene := models.Scene{
		ID:          strings.TrimPrefix(path, "/movie/"),
		SiteID:      siteID,
		StudioURL:   studioURL,
		Title:       d.title,
		URL:         sceneURL,
		Studio:      studioName,
		Description: d.description,
		Performers:  d.performers,
		Date:        d.date,
		Duration:    d.duration,
		Resolution:  d.resolution,
		Thumbnail:   absolutize(d.thumb, base),
		Preview:     absolutize(d.preview, base),
		ScrapedAt:   time.Now().UTC(),
	}
	// Prices are per format (4K, HD, DVD). The cheapest is what the scene
	// costs to obtain, so that is the snapshot; recording three would make the
	// history meaningless.
	if d.price > 0 {
		scene.AddPrice(models.PriceSnapshot{Date: scene.ScrapedAt, Regular: d.price})
	}
	return scene
}

func (s *Scraper) fetch(ctx context.Context, u string) ([]byte, error) {
	headers := httpx.BrowserHeaders(httpx.UserAgentFirefox)
	headers["Cookie"] = ageCookie
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		Method:  http.MethodGet,
		URL:     u,
		Headers: headers,
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}

// ---- movie page ----

type movieData struct {
	title       string
	description string
	performers  []string
	date        time.Time
	duration    int
	resolution  string
	thumb       string
	preview     string
	price       float64
}

var (
	scriptRe = regexp.MustCompile(`(?s)<script.*?</script>`)
	styleRe  = regexp.MustCompile(`(?s)<style.*?</style>`)
	h1Re     = regexp.MustCompile(`(?s)<h1>(.*?)</h1>`)
	castRe   = regexp.MustCompile(`(?s)<div id="cast">(.*?)</div>`)
	// The description block holds the synopsis paragraphs and then a `data`
	// line: "9 August 2026 • 3840x 2160 • 30 minutes • hd: 862.1MB • 4k: 3.3GB".
	descBlockRe = regexp.MustCompile(`(?s)<div id="description">(.*?)</div>`)
	dataLineRe  = regexp.MustCompile(`(?s)<p id="data">(.*?)</p>`)
	paraRe      = regexp.MustCompile(`(?s)<p>(.*?)</p>`)
	anchorRe    = regexp.MustCompile(`(?s)<a[^>]*>(.*?)</a>`)
	posterRe    = regexp.MustCompile(`poster="([^"]+)"`)
	sourceRe    = regexp.MustCompile(`<source[^>]+src="([^"]+\.mp4)"`)
	priceRe     = regexp.MustCompile(`\$([\d.]+)`)
	buyBoxRe    = regexp.MustCompile(`(?s)<div class="buybox">(.*?)</div>`)
	dateRe      = regexp.MustCompile(`(\d{1,2}\s+[A-Za-z]+\s+\d{4})`)
	minutesRe   = regexp.MustCompile(`(\d+)\s*minutes?`)
	resRe       = regexp.MustCompile(`(\d{3,4})\s*x\s*(\d{3,4})`)
)

func parseMovie(body string) movieData {
	body = styleRe.ReplaceAllString(scriptRe.ReplaceAllString(body, ""), "")

	d := movieData{
		title:   cleanText(firstSubmatch(h1Re, body)),
		thumb:   firstSubmatch(posterRe, body),
		preview: firstSubmatch(sourceRe, body),
	}
	for _, m := range anchorRe.FindAllStringSubmatch(firstSubmatch(castRe, body), -1) {
		if n := cleanText(m[1]); n != "" {
			d.performers = appendUnique(d.performers, n)
		}
	}

	block := firstSubmatch(descBlockRe, body)
	data := firstSubmatch(dataLineRe, block)
	if data == "" {
		// The description div closes at the first nested </p>, so the data
		// line can land outside it on some builds.
		data = firstSubmatch(dataLineRe, body)
	}
	// The synopsis is every paragraph in the block except the data line.
	var paras []string
	for _, m := range paraRe.FindAllStringSubmatch(block, -1) {
		if t := cleanText(m[1]); t != "" && t != cleanText(data) {
			paras = append(paras, t)
		}
	}
	d.description = strings.Join(paras, " ")

	data = cleanText(data)
	if v := firstSubmatch(dateRe, data); v != "" {
		if t, err := time.Parse("2 January 2006", strings.Join(strings.Fields(v), " ")); err == nil {
			d.date = t.UTC()
		}
	}
	if v := firstSubmatch(minutesRe, data); v != "" {
		if mins, err := strconv.Atoi(v); err == nil {
			d.duration = mins * 60
		}
	}
	if m := resRe.FindStringSubmatch(data); m != nil {
		d.resolution = m[1] + "x" + m[2]
	}

	// The buybox lists one price per format; the cheapest is what the scene
	// costs to obtain.
	for _, m := range priceRe.FindAllStringSubmatch(firstSubmatch(buyBoxRe, body), -1) {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil && v > 0 {
			if d.price == 0 || v < d.price {
				d.price = v
			}
		}
	}
	return d
}

// ---- helpers ----

var tagStripRe = regexp.MustCompile(`<[^>]*>`)

func cleanText(s string) string {
	s = tagStripRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, " ", " ")
	return strings.Join(strings.Fields(s), " ")
}

func appendUnique(list []string, v string) []string {
	for _, existing := range list {
		if existing == v {
			return list
		}
	}
	return append(list, v)
}

func firstSubmatch(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func absolutize(ref, base string) string {
	switch {
	case ref == "":
		return ""
	case strings.HasPrefix(ref, "http://"), strings.HasPrefix(ref, "https://"):
		return ref
	case strings.HasPrefix(ref, "/"):
		return base + ref
	default:
		return base + "/" + ref
	}
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
