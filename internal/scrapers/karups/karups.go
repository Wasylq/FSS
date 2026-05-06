package karups

import (
	"context"
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

type siteConfig struct {
	id      string
	base    string
	studio  string
	pattern string
	matchRe *regexp.Regexp
}

var sites = []siteConfig{
	{
		id:      "karupsow",
		base:    "https://www.karupsow.com",
		studio:  "Karups Older Women",
		pattern: "karupsow.com",
		matchRe: regexp.MustCompile(`^https?://(?:www\.)?karupsow\.com`),
	},
	{
		id:      "karupspc",
		base:    "https://www.karupspc.com",
		studio:  "Karups Private Collection",
		pattern: "karupspc.com",
		matchRe: regexp.MustCompile(`^https?://(?:www\.)?karupspc\.com`),
	},
	{
		id:      "karupsha",
		base:    "https://www.karupsha.com",
		studio:  "Karups Hometown Amateurs",
		pattern: "karupsha.com",
		matchRe: regexp.MustCompile(`^https?://(?:www\.)?karupsha\.com`),
	},
}

type Scraper struct {
	client *http.Client
	cfg    siteConfig
}

func newScraper(cfg siteConfig) *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second), cfg: cfg}
}

func init() {
	for _, cfg := range sites {
		scraper.Register(newScraper(cfg))
	}
}

func (s *Scraper) ID() string { return s.cfg.id }
func (s *Scraper) Patterns() []string {
	return []string{s.cfg.pattern, s.cfg.pattern + "/model/{slug}.html"}
}
func (s *Scraper) MatchesURL(u string) bool { return s.cfg.matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	if opts.Workers <= 0 {
		opts.Workers = 3
	}
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	cardRe     = regexp.MustCompile(`(?s)<div class="item-inside">\s*<a href="([^"]+)">\s*<div class="thumb">\s*<img src="([^"]+)"[^>]*>\s*<div class="meta">\s*<span class="title">([^<]+)</span>\s*<span class="date">([^<]+)</span>`)
	sceneIDRe  = regexp.MustCompile(`-(\d+)\.html`)
	maxPageRe  = regexp.MustCompile(`href="page(\d+)\.html"`)
	modelURLRe = regexp.MustCompile(`/model/[^/]+\.html`)

	videoSectionRe = regexp.MustCompile(`(?s)<div class="listing-videos[^"]*"[^>]*>(.*)</div>`)

	modelsRe    = regexp.MustCompile(`(?s)<span class="models">(.*?)</span>\s*</span>`)
	modelNameRe = regexp.MustCompile(`<a[^>]*>([^<]+)</a>`)

	ordinalRe = regexp.MustCompile(`(\d+)(st|nd|rd|th)`)
)

type listEntry struct {
	id        string
	url       string
	title     string
	thumbnail string
	date      time.Time
}

func parseListingPage(body []byte) []listEntry {
	matches := cardRe.FindAllSubmatch(body, -1)
	entries := make([]listEntry, 0, len(matches))
	for _, m := range matches {
		url := string(m[1])
		idMatch := sceneIDRe.FindStringSubmatch(url)
		if idMatch == nil {
			continue
		}
		entries = append(entries, listEntry{
			id:        idMatch[1],
			url:       url,
			title:     strings.TrimSpace(html.UnescapeString(string(m[3]))),
			thumbnail: string(m[2]),
			date:      parseDate(string(m[4])),
		})
	}
	return entries
}

func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	s = ordinalRe.ReplaceAllString(s, "$1")
	t, err := time.Parse("January 2, 2006", s)
	if err != nil {
		t, _ = time.Parse("Jan 2, 2006", s)
	}
	return t.UTC()
}

func estimateTotal(body []byte, perPage int) int {
	max := 1
	for _, m := range maxPageRe.FindAllSubmatch(body, -1) {
		n, _ := strconv.Atoi(string(m[1]))
		if n > max {
			max = n
		}
	}
	return max * perPage
}

func listingURL(base string, page int) string {
	if page <= 1 {
		return base + "/videos/"
	}
	return fmt.Sprintf("%s/videos/page%d.html", base, page)
}

func resolveBase(studioURL string, cfg siteConfig) string {
	if m := regexp.MustCompile(`^(https?://[^/]+)`).FindString(studioURL); m != "" {
		if !strings.Contains(m, "karups") {
			return m
		}
	}
	return cfg.base
}

func isModelURL(u string) bool { return modelURLRe.MatchString(u) }

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)
	if isModelURL(studioURL) {
		s.runModel(ctx, studioURL, opts, out)
	} else {
		s.runPaginated(ctx, studioURL, opts, out)
	}
}

func (s *Scraper) runModel(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	now := time.Now().UTC()

	body, err := s.fetchPage(ctx, studioURL)
	if err != nil {
		select {
		case out <- scraper.Error(err):
		case <-ctx.Done():
		}
		return
	}

	section := body
	if m := videoSectionRe.Find(body); m != nil {
		section = m
	}

	entries := parseListingPage(section)
	select {
	case out <- scraper.Progress(len(entries)):
	case <-ctx.Done():
		return
	}

	for _, e := range entries {
		if ctx.Err() != nil {
			return
		}
		if len(opts.KnownIDs) > 0 && opts.KnownIDs[e.id] {
			select {
			case out <- scraper.StoppedEarly():
			case <-ctx.Done():
			}
			return
		}
		if opts.Delay > 0 {
			select {
			case <-time.After(opts.Delay):
			case <-ctx.Done():
				return
			}
		}
		scene := s.processEntry(ctx, studioURL, now, e)
		select {
		case out <- scraper.Scene(scene):
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scraper) runPaginated(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	base := resolveBase(studioURL, s.cfg)
	now := time.Now().UTC()

	work := make(chan listEntry, opts.Workers)
	var wg sync.WaitGroup

	for i := 0; i < opts.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for entry := range work {
				if opts.Delay > 0 {
					select {
					case <-time.After(opts.Delay):
					case <-ctx.Done():
						return
					}
				}
				scene := s.processEntry(ctx, studioURL, now, entry)
				select {
				case out <- scraper.Scene(scene):
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	seen := make(map[string]bool)
	for page := 1; ; page++ {
		if ctx.Err() != nil {
			break
		}
		if page > 1 && opts.Delay > 0 {
			select {
			case <-time.After(opts.Delay):
			case <-ctx.Done():
			}
			if ctx.Err() != nil {
				break
			}
		}

		body, err := s.fetchPage(ctx, listingURL(base, page))
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			break
		}

		entries := parseListingPage(body)
		if len(entries) == 0 {
			break
		}

		if page == 1 {
			total := estimateTotal(body, len(entries))
			if total > 0 {
				select {
				case out <- scraper.Progress(total):
				case <-ctx.Done():
				}
			}
		}

		cancelled := false
		hitKnown := false
		for _, e := range entries {
			if seen[e.id] {
				continue
			}
			seen[e.id] = true
			if len(opts.KnownIDs) > 0 && opts.KnownIDs[e.id] {
				hitKnown = true
				break
			}
			select {
			case work <- e:
			case <-ctx.Done():
				cancelled = true
			}
			if cancelled {
				break
			}
		}
		if cancelled || hitKnown {
			if hitKnown {
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
			}
			break
		}
	}

	close(work)
	wg.Wait()
}

func (s *Scraper) processEntry(ctx context.Context, studioURL string, now time.Time, entry listEntry) models.Scene {
	scene := models.Scene{
		ID:        entry.id,
		SiteID:    s.cfg.id,
		StudioURL: studioURL,
		Title:     entry.title,
		Thumbnail: entry.thumbnail,
		URL:       entry.url,
		Date:      entry.date,
		Studio:    s.cfg.studio,
		ScrapedAt: now,
	}

	body, err := s.fetchPage(ctx, entry.url)
	if err != nil {
		return scene
	}

	if m := modelsRe.FindSubmatch(body); m != nil {
		for _, name := range modelNameRe.FindAllSubmatch(m[1], -1) {
			performer := strings.TrimSpace(html.UnescapeString(string(name[1])))
			if performer != "" {
				scene.Performers = append(scene.Performers, performer)
			}
		}
	}

	return scene
}

func (s *Scraper) fetchPage(ctx context.Context, url string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: url,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentChrome,
			"Cookie":     "warningHidden=hide",
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}
