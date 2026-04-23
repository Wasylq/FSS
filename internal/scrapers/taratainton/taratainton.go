package taratainton

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
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

type Scraper struct {
	client   *http.Client
	siteBase string
	headers  map[string]string
}

func New() *Scraper {
	return &Scraper{
		client:   httpx.NewClient(30 * time.Second),
		siteBase: "https://www.taratainton.com",
		headers: map[string]string{
			"User-Agent":      httpx.UserAgentFirefox,
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			"Accept-Language": "en-US,en;q=0.5",
		},
	}
}

func init() {
	scraper.Register(New())
}

func (s *Scraper) ID() string { return "taratainton" }

func (s *Scraper) Patterns() []string {
	return []string{"taratainton.com"}
}

var matchRe = regexp.MustCompile(`taratainton\.com`)

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	if opts.Workers <= 0 {
		opts.Workers = 3
	}
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// ---- sitemap ----

type urlset struct {
	URLs []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc string `xml:"loc"`
}

func (s *Scraper) fetchSitemap(ctx context.Context, sitemapURL string) ([]sitemapURL, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     sitemapURL,
		Headers: s.headers,
	})
	if err != nil {
		return nil, fmt.Errorf("fetching sitemap %s: %w", sitemapURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading sitemap: %w", err)
	}

	var us urlset
	if err := xml.Unmarshal(body, &us); err != nil {
		return nil, fmt.Errorf("parsing sitemap XML: %w", err)
	}
	return us.URLs, nil
}

// ---- worker pool ----

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	sitemaps := []string{
		s.siteBase + "/post-sitemap.xml",
		s.siteBase + "/post-sitemap2.xml",
	}

	var allURLs []sitemapURL
	for _, sm := range sitemaps {
		if ctx.Err() != nil {
			return
		}
		urls, err := s.fetchSitemap(ctx, sm)
		if err != nil {
			select {
			case out <- scraper.SceneResult{Err: err}:
			case <-ctx.Done():
			}
			return
		}
		allURLs = append(allURLs, urls...)
	}

	if len(allURLs) > 0 {
		select {
		case out <- scraper.SceneResult{Total: len(allURLs)}:
		case <-ctx.Done():
			return
		}
	}

	work := make(chan sitemapURL, opts.Workers)
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
				scene, skip, err := s.fetchAndParse(ctx, studioURL, entry.Loc)
				if err != nil {
					select {
					case out <- scraper.SceneResult{Err: err}:
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
				case out <- scraper.SceneResult{Scene: scene}:
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

func (s *Scraper) fetchAndParse(ctx context.Context, studioURL, pageURL string) (models.Scene, bool, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     pageURL,
		Headers: s.headers,
	})
	if err != nil {
		return models.Scene{}, false, fmt.Errorf("fetching %s: %w", pageURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.Scene{}, false, fmt.Errorf("reading %s: %w", pageURL, err)
	}

	return parsePage(studioURL, pageURL, body, time.Now().UTC())
}

// ---- parsing ----

var (
	titleRe       = regexp.MustCompile(`<title>([^<]+)</title>`)
	publishedRe   = regexp.MustCompile(`article:published_time"\s+content="([^"]+)"`)
	ogDescRe      = regexp.MustCompile(`og:description"\s+content="([^"]*)"`)
	ogImageRe     = regexp.MustCompile(`og:image"\s+content="([^"]+)"`)
	shortlinkRe   = regexp.MustCompile(`rel='shortlink'\s+href='[^?]*\?p=(\d+)'`)
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

	id := ""
	if m := shortlinkRe.FindSubmatch(body); m != nil {
		id = string(m[1])
	} else {
		id = slugFromURL(pageURL)
	}

	title := ""
	if m := titleRe.FindSubmatch(body); m != nil {
		title = html.UnescapeString(string(m[1]))
		title = strings.TrimSuffix(title, titleSuffix)
		title = strings.TrimSpace(title)
	}

	var date time.Time
	if m := publishedRe.FindSubmatch(body); m != nil {
		if t, err := time.Parse(time.RFC3339, string(m[1])); err == nil {
			date = t.UTC()
		}
	}

	description := ""
	if m := ogDescRe.FindSubmatch(body); m != nil {
		description = html.UnescapeString(string(m[1]))
	}

	thumbnail := ""
	if m := ogImageRe.FindSubmatch(body); m != nil {
		thumbnail = string(m[1])
	}

	price, _ := strconv.ParseFloat(string(plMatch[1]), 64)
	duration := parseDuration(string(plMatch[2]))

	resolution := ""
	var width, height int
	if m := resolutionRe.FindSubmatch(body); m != nil {
		h, _ := strconv.Atoi(string(m[1]))
		height = h
		width = videoWidth(h)
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
		Title:       title,
		URL:         pageURL,
		Date:        date,
		Description: description,
		Thumbnail:   thumbnail,
		Performers:  []string{"Tara Tainton"},
		Studio:      "Tara Tainton",
		Tags:        tags,
		Duration:    duration,
		Resolution:  resolution,
		Width:       width,
		Height:      height,
		ScrapedAt:   now,
	}

	scene.AddPrice(models.PriceSnapshot{
		Date:    now,
		Regular: price,
	})

	return scene, false, nil
}

func slugFromURL(pageURL string) string {
	pageURL = strings.TrimSuffix(pageURL, ".html")
	parts := strings.Split(strings.TrimRight(pageURL, "/"), "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return pageURL
}

func parseDuration(s string) int {
	parts := strings.Split(s, ":")
	total := 0
	for _, p := range parts {
		n, _ := strconv.Atoi(p)
		total = total*60 + n
	}
	return total
}

func videoWidth(height int) int {
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
