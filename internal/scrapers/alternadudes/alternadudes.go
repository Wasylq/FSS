package alternadudes

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

type Scraper struct {
	client *http.Client
}

func New() *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second)}
}

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return "alternadudes" }

func (s *Scraper) Patterns() []string {
	return []string{"alternadudes.com"}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?alternadudes\.com`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	if opts.Workers <= 0 {
		opts.Workers = 3
	}
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

const siteBase = "https://www.alternadudes.com"

var (
	cardRe    = regexp.MustCompile(`(?s)<div class="item col-lg-4[^"]*">\s*<div class="product-item">.*?</div>\s*</div>\s*</div>`)
	thumbRe   = regexp.MustCompile(`<img src="(/content/contentthumbs/[^"]+)"`)
	titleRe   = regexp.MustCompile(`<img[^>]+alt="([^"]+)"`)
	trailerRe = regexp.MustCompile(`href="(?:https?://(?:www\.)?alternadudes\.com)?(/trailers/[^"]+\.html)"`)
	contentID = regexp.MustCompile(`/(\d+)-\d+x\.jpg`)
	maxPageRe = regexp.MustCompile(`movies_(\d+)_d\.html`)

	detailDescRe = regexp.MustCompile(`(?s)<h4>([^<]+)</h4>`)
	detailTagsRe = regexp.MustCompile(`<meta\s+name="keywords"\s+content="([^"]*)"`)
	ogImageRe    = regexp.MustCompile(`<meta\s+property="og:image"\s+content="([^"]*)"`)
)

type listEntry struct {
	id         string
	title      string
	thumbnail  string
	trailerURL string
}

func parseListingPage(body []byte) []listEntry {
	cards := cardRe.FindAll(body, -1)
	entries := make([]listEntry, 0, len(cards))
	for _, card := range cards {
		block := string(card)

		var e listEntry

		if m := contentID.FindStringSubmatch(block); m != nil {
			e.id = m[1]
		}
		if e.id == "" {
			continue
		}

		if m := titleRe.FindStringSubmatch(block); m != nil {
			e.title = html.UnescapeString(strings.TrimSpace(m[1]))
		}

		if m := thumbRe.FindStringSubmatch(block); m != nil {
			e.thumbnail = siteBase + m[1]
		}

		if m := trailerRe.FindStringSubmatch(block); m != nil {
			e.trailerURL = m[1]
		}

		entries = append(entries, e)
	}
	return entries
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

func pageURL(base string, page int) string {
	if page <= 1 {
		return base + "/categories/movies.html"
	}
	return fmt.Sprintf("%s/categories/movies_%d_d.html", base, page)
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	base := siteBase
	if strings.HasPrefix(studioURL, "http") {
		if m := regexp.MustCompile(`^(https?://[^/]+)`).FindString(studioURL); m != "" {
			base = m
		}
	}

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
				scene := s.processEntry(ctx, base, studioURL, entry)
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
				break
			}
			if ctx.Err() != nil {
				break
			}
		}

		body, err := s.fetchPage(ctx, pageURL(base, page))
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

func (s *Scraper) processEntry(ctx context.Context, base, studioURL string, entry listEntry) models.Scene {
	now := time.Now().UTC()
	scene := models.Scene{
		ID:        entry.id,
		SiteID:    "alternadudes",
		StudioURL: studioURL,
		Title:     entry.title,
		Thumbnail: entry.thumbnail,
		Studio:    "AlternaDudes",
		ScrapedAt: now,
	}

	if entry.trailerURL != "" {
		scene.URL = base + entry.trailerURL
		body, err := s.fetchPage(ctx, scene.URL)
		if err == nil {
			if m := detailDescRe.FindSubmatch(body); m != nil {
				scene.Description = strings.TrimSpace(html.UnescapeString(string(m[1])))
			}
			if m := detailTagsRe.FindSubmatch(body); m != nil {
				for _, tag := range strings.Split(html.UnescapeString(string(m[1])), ",") {
					tag = strings.TrimSpace(tag)
					if tag != "" && tag != "..." {
						scene.Tags = append(scene.Tags, tag)
					}
				}
			}
			if m := ogImageRe.FindSubmatch(body); m != nil {
				scene.Thumbnail = html.UnescapeString(string(m[1]))
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
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}
