package auntjudys

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
	defaultBase = "https://www.auntjudysxxx.com"
	siteID      = "auntjudys"
)

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?auntjudysxxx\.com`)

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
	return []string{
		"auntjudysxxx.com",
		"auntjudysxxx.com/tour/categories/movies.html",
		"auntjudysxxx.com/tour/models/{slug}.html",
	}
}
func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

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
				scene, err := s.fetchDetail(ctx, ls, studioURL, opts.Delay)
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

	isModel := strings.Contains(studioURL, "/models/")

	go func() {
		defer close(work)
		if isModel {
			s.enqueueModelPage(ctx, studioURL, opts, out, work)
		} else {
			s.enqueueListingPages(ctx, opts, out, work)
		}
	}()

	wg.Wait()
}

func (s *Scraper) enqueueListingPages(ctx context.Context, opts scraper.ListOpts, out chan<- scraper.SceneResult, work chan<- listingScene) {
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

		var pageURL string
		if page == 1 {
			pageURL = s.base + "/tour/categories/movies.html"
		} else {
			pageURL = fmt.Sprintf("%s/tour/categories/movies_%d_d.html", s.base, page)
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
			total := estimateTotal(body, len(scenes))
			if total > 0 {
				select {
				case out <- scraper.Progress(total):
				case <-ctx.Done():
					return
				}
			}
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
	}
}

func (s *Scraper) enqueueModelPage(ctx context.Context, modelURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult, work chan<- listingScene) {
	body, err := s.fetchPage(ctx, modelURL)
	if err != nil {
		select {
		case out <- scraper.Error(err):
		case <-ctx.Done():
		}
		return
	}

	scenes := parseListingPage(body, s.base)
	if len(scenes) == 0 {
		return
	}

	select {
	case out <- scraper.Progress(len(scenes)):
	case <-ctx.Done():
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
}

type listingScene struct {
	id         string
	url        string
	date       time.Time
	performers []string
	thumb      string
}

var (
	blockRe     = regexp.MustCompile(`(?s)class="update_details" data-setid="(\d+)"`)
	vidLinkRe   = regexp.MustCompile(`href="((?:https?://[^"]*|/[^"]*)_vids\.html)"`)
	dateRe      = regexp.MustCompile(`(?s)class="cell update_date">\s*(.*?)\s*</div>`)
	dateValRe   = regexp.MustCompile(`(\d{2}/\d{2}/\d{4})`)
	modelLinkRe = regexp.MustCompile(`href="[^"]*models/[^"]*">([^<]+)</a>`)
	thumbRe     = regexp.MustCompile(`src0_1x="([^"]+)"`)
	maxPageRe   = regexp.MustCompile(`movies_(\d+)_d\.html`)

	titleRe       = regexp.MustCompile(`(?s)class="title_bar_hilite">(.*?)</(?:span|div)>`)
	descRe        = regexp.MustCompile(`(?s)class="update_description">(.*?)</(?:span|div)>`)
	tagsRe        = regexp.MustCompile(`(?s)class="update_tags">(.*?)</(?:span|div)>`)
	tagLinkRe     = regexp.MustCompile(`>([^<]+)</a>`)
	modelsBlockRe = regexp.MustCompile(`(?s)class="update_models">(.*?)</(?:span|div)>`)
	countsRe      = regexp.MustCompile(`(?s)class="update_counts">(.*?)</div>`)
	durationRe    = regexp.MustCompile(`(\d+)\s*(?:&nbsp;)*\s*min`)
)

func parseListingPage(body []byte, base string) []listingScene {
	page := string(body)
	locs := blockRe.FindAllStringSubmatchIndex(page, -1)
	scenes := make([]listingScene, 0, len(locs))

	for i, loc := range locs {
		id := page[loc[2]:loc[3]]
		start := loc[0]
		end := len(page)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		block := page[start:end]

		ls := listingScene{id: id}

		if m := vidLinkRe.FindStringSubmatch(block); m != nil {
			href := m[1]
			if strings.HasPrefix(href, "/") {
				href = base + href
			}
			ls.url = href
		}

		if m := dateRe.FindStringSubmatch(block); m != nil {
			raw := m[1]
			if dm := dateValRe.FindString(raw); dm != "" {
				if t, err := time.Parse("01/02/2006", dm); err == nil {
					ls.date = t.UTC()
				}
			}
		}

		for _, m := range modelLinkRe.FindAllStringSubmatch(block, -1) {
			name := strings.TrimSpace(html.UnescapeString(m[1]))
			if name != "" {
				ls.performers = append(ls.performers, name)
			}
		}

		if m := thumbRe.FindStringSubmatch(block); m != nil {
			thumb := m[1]
			if strings.HasPrefix(thumb, "/") {
				thumb = base + thumb
			}
			ls.thumb = thumb
		}

		if ls.url == "" {
			continue
		}

		scenes = append(scenes, ls)
	}
	return scenes
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

type detailData struct {
	title       string
	description string
	tags        []string
	performers  []string
	duration    int
}

func parseDetailPage(body []byte) detailData {
	var d detailData
	page := string(body)

	if m := titleRe.FindStringSubmatch(page); m != nil {
		d.title = strings.TrimSpace(html.UnescapeString(m[1]))
	}

	if m := descRe.FindStringSubmatch(page); m != nil {
		d.description = strings.TrimSpace(html.UnescapeString(m[1]))
	}

	if m := tagsRe.FindStringSubmatch(page); m != nil {
		for _, tm := range tagLinkRe.FindAllStringSubmatch(m[1], -1) {
			tag := strings.TrimSpace(html.UnescapeString(tm[1]))
			if tag != "" {
				d.tags = append(d.tags, tag)
			}
		}
	}

	if m := modelsBlockRe.FindStringSubmatch(page); m != nil {
		for _, nm := range modelLinkRe.FindAllStringSubmatch(m[1], -1) {
			name := strings.TrimSpace(html.UnescapeString(nm[1]))
			if name != "" {
				d.performers = append(d.performers, name)
			}
		}
	}

	if m := countsRe.FindStringSubmatch(page); m != nil {
		if dm := durationRe.FindStringSubmatch(m[1]); dm != nil {
			mins, _ := strconv.Atoi(dm[1])
			d.duration = mins * 60
		}
	}

	return d
}

func (s *Scraper) fetchDetail(ctx context.Context, ls listingScene, studioURL string, delay time.Duration) (models.Scene, error) {
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
		StudioURL:  studioURL,
		URL:        ls.url,
		Date:       ls.date,
		Performers: ls.performers,
		Thumbnail:  ls.thumb,
		Studio:     "Aunt Judy's",
		ScrapedAt:  now,
	}

	if ls.url != "" {
		body, err := s.fetchPage(ctx, ls.url)
		if err != nil {
			return models.Scene{}, fmt.Errorf("detail %s: %w", ls.id, err)
		}
		detail := parseDetailPage(body)
		scene.Title = detail.title
		scene.Description = detail.description
		scene.Tags = detail.tags
		scene.Duration = detail.duration
		if len(detail.performers) > 0 {
			scene.Performers = detail.performers
		}
	}

	return scene, nil
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
