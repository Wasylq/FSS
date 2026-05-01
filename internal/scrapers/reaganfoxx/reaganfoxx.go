package reaganfoxx

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

const (
	defaultBase = "https://www.reaganfoxx.com"
	siteID      = "reaganfoxx"
)

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?reaganfoxx\.com`)

type Scraper struct {
	client *http.Client
	base   string
}

func New() *Scraper {
	return &Scraper{
		client: httpx.NewClient(30 * time.Second),
		base:   defaultBase,
	}
}

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }
func (s *Scraper) Patterns() []string {
	return []string{"reaganfoxx.com", "reaganfoxx.com/scenes/{id}/{slug}.html"}
}
func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	listingURL := studioURL
	if !strings.Contains(studioURL, "/scenes/") {
		listingURL = s.base + "/scenes/673608/reagan-foxx-streaming-pornstar-videos.html"
	}

	workers := opts.Workers
	if workers <= 0 {
		workers = 4
	}

	work := make(chan listingScene)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ls := range work {
				scene, err := s.fetchDetail(ctx, ls, opts.Delay)
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

	go func() {
		defer close(work)
		s.enqueuePages(ctx, listingURL, opts, out, work)
	}()

	wg.Wait()
}

func (s *Scraper) enqueuePages(ctx context.Context, listingURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult, work chan<- listingScene) {
	for page := 1; ; page++ {
		if ctx.Err() != nil {
			return
		}
		if page > 1 && opts.Delay > 0 {
			select {
			case <-time.After(opts.Delay):
			case <-ctx.Done():
				return
			}
		}

		pageURL := listingURL
		if page > 1 {
			sep := "?"
			if strings.Contains(listingURL, "?") {
				sep = "&"
			}
			pageURL = fmt.Sprintf("%s%spage=%d", listingURL, sep, page)
		}

		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		scenes := parseListingPage(body, s.base)
		if len(scenes) == 0 {
			return
		}

		if page == 1 {
			total := extractTotal(body)
			if total > 0 {
				select {
				case out <- scraper.Progress(total):
				case <-ctx.Done():
					return
				}
			}
		}

		if !hasPagination(body) {
			return
		}

		for _, ls := range scenes {
			if opts.KnownIDs[ls.id] {
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
				return
			}
			select {
			case work <- ls:
			case <-ctx.Done():
				return
			}
		}

		if !hasNextPage(body, page) {
			return
		}
	}
}

type listingScene struct {
	id         string
	url        string
	title      string
	performers []string
	duration   int
	thumb      string
}

var (
	widgetRe     = regexp.MustCompile(`(?s)<article class="scene-widget[^"]*"\s*data-scene-id="(\d+)".*?</article>`)
	sceneLinkRe  = regexp.MustCompile(`<a class="scene-title"\s+href="([^"]+)"`)
	titleRe      = regexp.MustCompile(`<a class="scene-title"[^>]*>\s*<h6>\s*(.*?)\s*</h6>`)
	performerRe  = regexp.MustCompile(`<p class="scene-performer-names">\s*(.*?)\s*</p>`)
	durationRe   = regexp.MustCompile(`<p class="scene-length">\s*(\d+)\s*min`)
	thumbRe      = regexp.MustCompile(`data-src="(https://caps1cdn[^"]+)"`)
	totalRe      = regexp.MustCompile(`<h4>(\d+)\s+Results</h4>`)
	paginationRe = regexp.MustCompile(`class="pagination`)
	pageNumRe    = regexp.MustCompile(`\?page=(\d+)`)

	releaseDateRe = regexp.MustCompile(`(?s)Released:</span>\s*(.*?)(?:</div>|<br)`)
	tagsBlockRe   = regexp.MustCompile(`(?s)Tags:</span>(.*?)</div>`)
	tagLinkRe     = regexp.MustCompile(`>([^<]+)</a>`)
	studioRe      = regexp.MustCompile(`(?s)Studio:</span>\s*<span>(.*?)</span>`)
	scenePriceRe  = regexp.MustCompile(`\$(\d+\.\d+)`)
)

func parseListingPage(body []byte, base string) []listingScene {
	matches := widgetRe.FindAllSubmatch(body, -1)
	scenes := make([]listingScene, 0, len(matches))

	for _, m := range matches {
		block := string(m[0])
		id := string(m[1])

		ls := listingScene{id: id}

		if sm := sceneLinkRe.FindStringSubmatch(block); sm != nil {
			href := sm[1]
			if strings.HasPrefix(href, "/") {
				href = base + href
			}
			ls.url = href
		}

		if sm := titleRe.FindStringSubmatch(block); sm != nil {
			ls.title = strings.TrimSpace(html.UnescapeString(sm[1]))
		}

		if sm := performerRe.FindStringSubmatch(block); sm != nil {
			raw := strings.TrimSpace(html.UnescapeString(sm[1]))
			for _, name := range strings.Split(raw, ",") {
				name = strings.TrimSpace(name)
				if name != "" {
					ls.performers = append(ls.performers, name)
				}
			}
		}

		if sm := durationRe.FindStringSubmatch(block); sm != nil {
			mins, _ := strconv.Atoi(sm[1])
			ls.duration = mins * 60
		}

		if sm := thumbRe.FindStringSubmatch(block); sm != nil {
			ls.thumb = sm[1]
		}

		scenes = append(scenes, ls)
	}
	return scenes
}

func extractTotal(body []byte) int {
	if m := totalRe.FindSubmatch(body); m != nil {
		n, _ := strconv.Atoi(string(m[1]))
		return n
	}
	return 0
}

func hasPagination(body []byte) bool {
	return paginationRe.Match(body)
}

func hasNextPage(body []byte, current int) bool {
	for _, m := range pageNumRe.FindAllSubmatch(body, -1) {
		n, _ := strconv.Atoi(string(m[1]))
		if n > current {
			return true
		}
	}
	return false
}

type detailData struct {
	date   time.Time
	tags   []string
	studio string
	price  float64
}

func parseDetailPage(body []byte) detailData {
	var d detailData
	page := string(body)

	if m := releaseDateRe.FindStringSubmatch(page); m != nil {
		raw := strings.TrimSpace(m[1])
		if t, err := time.Parse("January 2, 2006", raw); err == nil {
			d.date = t.UTC()
		} else if t, err := time.Parse("Jan 2, 2006", raw); err == nil {
			d.date = t.UTC()
		}
	}

	if m := tagsBlockRe.FindStringSubmatch(page); m != nil {
		for _, tm := range tagLinkRe.FindAllStringSubmatch(m[1], -1) {
			tag := strings.TrimSpace(html.UnescapeString(tm[1]))
			if tag != "" {
				d.tags = append(d.tags, tag)
			}
		}
	}

	if m := studioRe.FindStringSubmatch(page); m != nil {
		d.studio = strings.TrimSpace(html.UnescapeString(m[1]))
	}

	idx := strings.Index(page, "Buy This Scene")
	if idx > 0 {
		start := idx - 500
		if start < 0 {
			start = 0
		}
		ctx := page[start:idx]
		prices := scenePriceRe.FindAllStringSubmatch(ctx, -1)
		if len(prices) > 0 {
			d.price, _ = strconv.ParseFloat(prices[len(prices)-1][1], 64)
		}
	}

	return d
}

func (s *Scraper) fetchDetail(ctx context.Context, ls listingScene, delay time.Duration) (models.Scene, error) {
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return models.Scene{}, ctx.Err()
		}
	}

	now := time.Now().UTC()
	scene := models.Scene{
		ID:         ls.id,
		SiteID:     siteID,
		StudioURL:  s.base,
		Title:      ls.title,
		URL:        ls.url,
		Duration:   ls.duration,
		Performers: ls.performers,
		Thumbnail:  ls.thumb,
		ScrapedAt:  now,
	}

	if ls.url != "" {
		body, err := s.fetchPage(ctx, ls.url)
		if err != nil {
			return models.Scene{}, fmt.Errorf("detail %s: %w", ls.id, err)
		}
		detail := parseDetailPage(body)
		scene.Date = detail.date
		scene.Tags = detail.tags
		if detail.studio != "" {
			scene.Studio = detail.studio
		}
		if detail.price > 0 {
			scene.AddPrice(models.PriceSnapshot{
				Date:    now,
				Regular: detail.price,
			})
		}
	}

	return scene, nil
}

func (s *Scraper) fetchPage(ctx context.Context, url string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: url,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentChrome,
			"Cookie":     "AgeConfirmed=true",
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}
