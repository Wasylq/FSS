package dorcelclub

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/http/cookiejar"
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
	siteBase     = "https://www.dorcelclub.com"
	siteID       = "dorcelclub"
	studioName   = "Dorcel Club"
	defaultDelay = 500 * time.Millisecond
)

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?dorcelclub\.com`)

type Scraper struct {
	client *http.Client
}

func New() *Scraper {
	jar, _ := cookiejar.New(nil)
	c := httpx.NewClient(30 * time.Second)
	c.Jar = jar
	return &Scraper{client: c}
}

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		"dorcelclub.com",
		"dorcelclub.com/en/pornstar/{slug}",
		"dorcelclub.com/en/collection/{slug}",
		"dorcelclub.com/en/fantasmes/{slug}",
		"dorcelclub.com/en/porn-movie/{slug}",
	}
}

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

type workItem struct {
	url        string
	id         string
	title      string
	performers []string
	thumb      string
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	delay := opts.Delay
	if delay == 0 {
		delay = defaultDelay
	}
	workers := opts.Workers
	if workers <= 0 {
		workers = 4
	}

	if err := s.initSession(ctx); err != nil {
		select {
		case out <- scraper.Error(fmt.Errorf("session init: %w", err)):
		case <-ctx.Done():
		}
		return
	}

	work := make(chan workItem)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range work {
				scene, err := s.fetchDetail(ctx, item, studioURL, delay)
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

	s.produceListing(ctx, studioURL, opts, out, work, delay)
	close(work)
	wg.Wait()
}

func (s *Scraper) initSession(ctx context.Context) error {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     siteBase + "/en/",
		Headers: map[string]string{"User-Agent": httpx.UserAgentFirefox},
	})
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

var (
	pornstarRe   = regexp.MustCompile(`/en/pornstar(?:-men)?/([a-zA-Z0-9-]+)`)
	collectionRe = regexp.MustCompile(`/en/collection/([a-zA-Z0-9-]+)`)
	fantasmeRe   = regexp.MustCompile(`/en/fantasmes/([a-zA-Z0-9-]+)`)
	movieRe      = regexp.MustCompile(`/en/porn-movie/([a-zA-Z0-9-]+)`)
)

func (s *Scraper) produceListing(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult, work chan<- workItem, delay time.Duration) {
	if pornstarRe.MatchString(studioURL) || movieRe.MatchString(studioURL) {
		s.scrapeHTMLPage(ctx, studioURL, opts, out, work)
		return
	}
	if m := collectionRe.FindStringSubmatch(studioURL); m != nil {
		s.paginateAJAX(ctx, fmt.Sprintf("/collection/%s/more", m[1]), "new", opts, out, work, delay)
		return
	}
	if m := fantasmeRe.FindStringSubmatch(studioURL); m != nil {
		s.paginateAJAX(ctx, fmt.Sprintf("/fantasmes/%s/more", m[1]), "new", opts, out, work, delay)
		return
	}
	s.paginateAJAX(ctx, "/scene/list/more/", "new", opts, out, work, delay)
}

func (s *Scraper) scrapeHTMLPage(ctx context.Context, pageURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult, work chan<- workItem) {
	body, err := s.fetchPage(ctx, pageURL)
	if err != nil {
		select {
		case out <- scraper.Error(err):
		case <-ctx.Done():
		}
		return
	}

	items := parseSceneCards(body)
	if len(items) > 0 {
		select {
		case out <- scraper.Progress(len(items)):
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
		case work <- item:
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scraper) paginateAJAX(ctx context.Context, basePath, sorting string, opts scraper.ListOpts, out chan<- scraper.SceneResult, work chan<- workItem, delay time.Duration) {
	totalSent := false
	for page := 1; ; page++ {
		if ctx.Err() != nil {
			return
		}

		if page > 1 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}
		}

		ajaxURL := fmt.Sprintf("%s%s?lang=en&sorting=%s&page=%d", siteBase, basePath, sorting, page)
		body, err := s.fetchAJAX(ctx, ajaxURL)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		items := parseSceneCards(body)
		if len(items) == 0 {
			return
		}

		hasNext := btnMoreRe.MatchString(body)
		if !totalSent {
			select {
			case out <- scraper.Progress(0):
			case <-ctx.Done():
				return
			}
			totalSent = true
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
			case work <- item:
			case <-ctx.Done():
				return
			}
		}

		if !hasNext {
			return
		}
	}
}

func (s *Scraper) fetchPage(ctx context.Context, pageURL string) (string, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     pageURL,
		Headers: map[string]string{"User-Agent": httpx.UserAgentFirefox},
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (s *Scraper) fetchAJAX(ctx context.Context, ajaxURL string) (string, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: ajaxURL,
		Headers: map[string]string{
			"User-Agent":       httpx.UserAgentFirefox,
			"X-Requested-With": "XMLHttpRequest",
		},
		Method: "POST",
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

var (
	sceneCardRe = regexp.MustCompile(`(?s)<div class="scene thumbnail[^"]*">.*?</div>\s*</div>`)
	sceneURLRe  = regexp.MustCompile(`<a href="/en/scene/(\d+)/([^"]+)" class="thumb">`)
	titleRe     = regexp.MustCompile(`(?s)<a href="/en/scene/\d+/[^"]+" class="title">\s*(.*?)\s*</a>`)
	actorsRe    = regexp.MustCompile(`(?s)<div class="actors">(.*?)</div>`)
	actorRe     = regexp.MustCompile(`>([^<]+)</a>`)
	thumbRe     = regexp.MustCompile(`<img[^>]+data-src="([^"]+)"[^>]+alt=`)
	btnMoreRe   = regexp.MustCompile(`class="btn-more"`)
)

func parseSceneCards(page string) []workItem {
	cards := sceneCardRe.FindAllString(page, -1)
	seen := map[string]bool{}
	var items []workItem

	for _, card := range cards {
		m := sceneURLRe.FindStringSubmatch(card)
		if m == nil {
			continue
		}
		id := m[1]
		if seen[id] {
			continue
		}
		seen[id] = true

		item := workItem{
			id:  id,
			url: siteBase + "/en/scene/" + id + "/" + m[2],
		}

		if tm := titleRe.FindStringSubmatch(card); tm != nil {
			item.title = html.UnescapeString(strings.TrimSpace(tm[1]))
		}

		if am := actorsRe.FindStringSubmatch(card); am != nil {
			for _, pm := range actorRe.FindAllStringSubmatch(am[1], -1) {
				item.performers = append(item.performers, strings.TrimSpace(pm[1]))
			}
		}

		if tm := thumbRe.FindStringSubmatch(card); tm != nil {
			item.thumb = tm[1]
		}

		items = append(items, item)
	}
	return items
}

var (
	detailDateRe     = regexp.MustCompile(`class="publish_date">([^<]+)<`)
	detailDurationRe = regexp.MustCompile(`class="duration">(\d+)m(\d+)<`)
	detailDescRe     = regexp.MustCompile(`(?s)<span class="full">(.*?)</span>`)
	detailMovieRe    = regexp.MustCompile(`(?s)<span class="movie">.*?<a href="/en/porn-movie/[^"]*">([^<]+)</a>`)
	detailDirectorRe = regexp.MustCompile(`class="director">[^:]*:\s*([^<]+)<`)
	detailActressRe  = regexp.MustCompile(`(?s)<div class="actress">(.*?)</div>`)
)

func (s *Scraper) fetchDetail(ctx context.Context, item workItem, studioURL string, delay time.Duration) (models.Scene, error) {
	select {
	case <-time.After(delay):
	case <-ctx.Done():
		return models.Scene{}, ctx.Err()
	}

	body, err := s.fetchPage(ctx, item.url)
	if err != nil {
		return models.Scene{}, fmt.Errorf("detail %s: %w", item.url, err)
	}

	var date time.Time
	if m := detailDateRe.FindStringSubmatch(body); m != nil {
		date, _ = time.Parse("January 02, 2006", strings.TrimSpace(m[1]))
	}

	var duration int
	if m := detailDurationRe.FindStringSubmatch(body); m != nil {
		mins, _ := strconv.Atoi(m[1])
		secs, _ := strconv.Atoi(m[2])
		duration = mins*60 + secs
	}

	var description string
	if m := detailDescRe.FindStringSubmatch(body); m != nil {
		description = stripTags(strings.TrimSpace(m[1]))
	}

	var series string
	if m := detailMovieRe.FindStringSubmatch(body); m != nil {
		series = strings.TrimSpace(m[1])
	}

	var director string
	if m := detailDirectorRe.FindStringSubmatch(body); m != nil {
		director = strings.TrimSpace(m[1])
	}

	if am := detailActressRe.FindStringSubmatch(body); am != nil {
		var performers []string
		for _, pm := range actorRe.FindAllStringSubmatch(am[1], -1) {
			performers = append(performers, strings.TrimSpace(pm[1]))
		}
		if len(performers) > 0 {
			item.performers = performers
		}
	}

	now := time.Now().UTC()
	return models.Scene{
		ID:          item.id,
		SiteID:      siteID,
		StudioURL:   studioURL,
		Title:       item.title,
		URL:         item.url,
		Date:        date.UTC(),
		Description: description,
		Thumbnail:   item.thumb,
		Performers:  item.performers,
		Director:    director,
		Studio:      studioName,
		Series:      series,
		Duration:    duration,
		ScrapedAt:   now,
	}, nil
}

var tagStripRe = regexp.MustCompile(`<[^>]*>`)

func stripTags(s string) string {
	return strings.TrimSpace(tagStripRe.ReplaceAllString(s, " "))
}
