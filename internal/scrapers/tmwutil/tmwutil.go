package tmwutil

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

type SiteConfig struct {
	SiteID     string
	Slug       string // TMW network slug, e.g. "anal-angels" (used in teenmegaworld.net URLs)
	Domain     string // subsite domain, e.g. "anal-angels.com"
	StudioName string
}

type Scraper struct {
	client *http.Client
	base   string
	Config SiteConfig
}

func NewScraper(cfg SiteConfig) *Scraper {
	return &Scraper{
		client: httpx.NewClient(30 * time.Second),
		base:   "https://" + cfg.Domain,
		Config: cfg,
	}
}

func (s *Scraper) ID() string { return s.Config.SiteID }

func (s *Scraper) Patterns() []string {
	return []string{
		s.Config.Domain + "/categories/movies",
		s.Config.Domain + "/trailers/",
		"teenmegaworld.net/categories/" + s.Config.Slug,
	}
}

func (s *Scraper) MatchesURL(u string) bool {
	lower := strings.ToLower(u)
	domain := strings.ToLower(s.Config.Domain)
	slug := strings.ToLower(s.Config.Slug)
	if strings.Contains(lower, "://"+domain) || strings.Contains(lower, "://www."+domain) {
		return true
	}
	return strings.Contains(lower, "teenmegaworld.net/categories/"+slug)
}

func (s *Scraper) ListScenes(ctx context.Context, _ string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, opts, out)
	return out, nil
}

func (s *Scraper) run(ctx context.Context, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	workers := opts.Workers
	if workers <= 0 {
		workers = 4
	}

	type detailWork struct {
		listing listingItem
	}

	work := make(chan detailWork)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for dw := range work {
				scene, err := s.fetchDetail(ctx, dw.listing, opts.Delay)
				if err != nil {
					select {
					case out <- scraper.Error(err):
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

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(work)

		for page := 1; ; page++ {
			if ctx.Err() != nil {
				return
			}

			url := fmt.Sprintf("%s/categories/movies_%d_d.html", s.base, page)
			body, err := s.fetchPage(ctx, url)
			if err != nil {
				select {
				case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
				case <-ctx.Done():
				}
				return
			}

			items := parseListingPage(body, s.base)
			if len(items) == 0 {
				return
			}

			if page == 1 {
				total := parseTotal(body)
				if total == 0 {
					total = len(items)
				}
				select {
				case out <- scraper.Progress(total):
				case <-ctx.Done():
					return
				}
			}

			for _, item := range items {
				if opts.KnownIDs[item.id] {
					select {
					case out <- scraper.StoppedEarly():
					case <-ctx.Done():
					}
					return
				}
				select {
				case work <- detailWork{listing: item}:
				case <-ctx.Done():
					return
				}
			}

			select {
			case <-time.After(opts.Delay):
			case <-ctx.Done():
				return
			}
		}
	}()

	wg.Wait()
}

type listingItem struct {
	id         string
	title      string
	date       time.Time
	performers []string
	thumb      string
	url        string
}

var (
	cardRe  = regexp.MustCompile(`(?s)<a\s+class="thumb\s+thumb-video[^"]*"\s+href="([^"]+)"[^>]*>(.+?)</a>`)
	slugRe  = regexp.MustCompile(`/trailers/([^/.]+)\.html`)
	titleRe = regexp.MustCompile(`class="thumb__title-link"[^>]*title="([^"]+)"`)
	dateRe  = regexp.MustCompile(`<time[^>]*datetime="([^"]+)"`)
	actorRe = regexp.MustCompile(`class="actor__name"[^>]*title="([^"]+)"`)
	thumbRe = regexp.MustCompile(`srcset="[^"]*?([^\s,"]+)\s+2x`)
	totalRe = regexp.MustCompile(`class="list-page-heading__count[^"]*">\s*([\d,]+)\s*</span>`)
)

func parseListingPage(body []byte, base string) []listingItem {
	cards := cardRe.FindAllSubmatch(body, -1)
	seen := map[string]bool{}
	var items []listingItem

	for _, c := range cards {
		href := string(c[1])
		card := c[2]

		sm := slugRe.FindStringSubmatch(href)
		if sm == nil {
			continue
		}
		id := sm[1]
		if seen[id] {
			continue
		}
		seen[id] = true

		item := listingItem{
			id:  id,
			url: href,
		}

		if m := titleRe.FindSubmatch(card); m != nil {
			item.title = strings.TrimSpace(html.UnescapeString(string(m[1])))
		}

		if m := dateRe.FindSubmatch(card); m != nil {
			if t, err := time.Parse("2006-01-02T15:04:05", string(m[1])); err == nil {
				item.date = t.UTC()
			}
		}

		actorMatches := actorRe.FindAllSubmatch(card, -1)
		for _, m := range actorMatches {
			name := strings.TrimSpace(html.UnescapeString(string(m[1])))
			if name != "" {
				item.performers = append(item.performers, name)
			}
		}

		if m := thumbRe.FindSubmatch(card); m != nil {
			thumb := string(m[1])
			if !strings.HasPrefix(thumb, "http") {
				thumb = base + "/" + strings.TrimPrefix(thumb, "/")
			}
			item.thumb = thumb
		}

		items = append(items, item)
	}
	return items
}

func parseTotal(body []byte) int {
	m := totalRe.FindSubmatch(body)
	if m == nil {
		return 0
	}
	s := strings.ReplaceAll(string(m[1]), ",", "")
	n, _ := strconv.Atoi(s)
	return n
}

var (
	detailDescRe = regexp.MustCompile(`(?s)<p\s+class="video-description-text"[^>]*>(.*?)</p>`)
	detailDurRe  = regexp.MustCompile(`<meta\s+property="video:duration"\s+content="(\d+)"`)
	detailTagRe  = regexp.MustCompile(`<meta\s+property="video:tag"\s+content="([^"]*)"`)
	detailPerfRe = regexp.MustCompile(`class="video-actor-link[^"]*"[^>]*title="([^"]+)"`)
)

func (s *Scraper) fetchDetail(ctx context.Context, item listingItem, delay time.Duration) (models.Scene, error) {
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return models.Scene{}, ctx.Err()
		}
	}

	body, err := s.fetchPage(ctx, item.url)
	if err != nil {
		return models.Scene{}, fmt.Errorf("detail %s: %w", item.id, err)
	}

	scene := models.Scene{
		ID:         item.id,
		SiteID:     s.Config.SiteID,
		StudioURL:  s.base + "/categories/movies.html",
		Title:      item.title,
		URL:        item.url,
		Thumbnail:  item.thumb,
		Studio:     s.Config.StudioName,
		Date:       item.date,
		Performers: item.performers,
		ScrapedAt:  time.Now().UTC(),
	}

	if m := detailDescRe.FindSubmatch(body); m != nil {
		scene.Description = strings.TrimSpace(html.UnescapeString(string(m[1])))
	}

	if m := detailDurRe.FindSubmatch(body); m != nil {
		dur, _ := strconv.Atoi(string(m[1]))
		scene.Duration = dur
	}

	tagMatches := detailTagRe.FindAllSubmatch(body, -1)
	seenTag := map[string]bool{}
	for _, m := range tagMatches {
		tag := strings.TrimSpace(html.UnescapeString(string(m[1])))
		if tag != "" && !seenTag[tag] {
			seenTag[tag] = true
			scene.Tags = append(scene.Tags, tag)
		}
	}

	perfMatches := detailPerfRe.FindAllSubmatch(body, -1)
	if len(perfMatches) > 0 {
		scene.Performers = nil
		seen := map[string]bool{}
		for _, m := range perfMatches {
			name := strings.TrimSpace(html.UnescapeString(string(m[1])))
			if name != "" && !seen[name] {
				seen[name] = true
				scene.Performers = append(scene.Performers, name)
			}
		}
	}

	return scene, nil
}

func (s *Scraper) fetchPage(ctx context.Context, url string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: url,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentFirefox,
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}
