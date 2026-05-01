package purecfnm

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const (
	defaultBase  = "https://www.purecfnm.com"
	siteID       = "purecfnm"
	defaultDelay = 500 * time.Millisecond
)

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?purecfnm\.com`)

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
		"purecfnm.com",
		"purecfnm.com/categories/{slug}_{page}_d.html",
		"purecfnm.com/models/{slug}.html",
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

	delay := opts.Delay
	if delay == 0 {
		delay = defaultDelay
	}

	now := time.Now().UTC()

	if strings.Contains(studioURL, "/models/") {
		s.scrapeModelPage(ctx, studioURL, opts, out, now)
	} else {
		s.scrapeListingPages(ctx, studioURL, delay, opts, out, now)
	}
}

// --- listing pages ---

var catSlugRe = regexp.MustCompile(`/categories/([^_/]+)_\d+_d\.html`)

func extractSlug(studioURL string) string {
	if m := catSlugRe.FindStringSubmatch(studioURL); m != nil {
		return m[1]
	}
	return "movies"
}

func (s *Scraper) scrapeListingPages(ctx context.Context, studioURL string, delay time.Duration, opts scraper.ListOpts, out chan<- scraper.SceneResult, now time.Time) {
	slug := extractSlug(studioURL)

	for page := 1; ; page++ {
		if ctx.Err() != nil {
			return
		}
		if page > 1 && delay > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}
		}

		pageURL := fmt.Sprintf("%s/categories/%s_%d_d.html", s.base, slug, page)

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

		for _, item := range scenes {
			if opts.KnownIDs[item.id] {
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
				return
			}
			select {
			case out <- scraper.Scene(item.toScene(studioURL, s.base, now)):
			case <-ctx.Done():
				return
			}
		}
	}
}

// --- model page ---

func (s *Scraper) scrapeModelPage(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult, now time.Time) {
	body, err := s.fetchPage(ctx, studioURL)
	if err != nil {
		select {
		case out <- scraper.Error(err):
		case <-ctx.Done():
		}
		return
	}

	scenes := parseModelPage(body, s.base)
	if len(scenes) == 0 {
		return
	}

	select {
	case out <- scraper.Progress(len(scenes)):
	case <-ctx.Done():
		return
	}

	for _, item := range scenes {
		if opts.KnownIDs[item.id] {
			select {
			case out <- scraper.StoppedEarly():
			case <-ctx.Done():
			}
			return
		}
		select {
		case out <- scraper.Scene(item.toScene(studioURL, s.base, now)):
		case <-ctx.Done():
			return
		}
	}
}

// --- parsing ---

type sceneItem struct {
	id          string
	title       string
	performers  []string
	description string
	tags        []string
	thumb       string
	date        time.Time
	duration    int
}

func (item sceneItem) toScene(studioURL, base string, now time.Time) models.Scene {
	return models.Scene{
		ID:          item.id,
		SiteID:      siteID,
		StudioURL:   studioURL,
		Title:       item.title,
		URL:         base + "/#" + item.id,
		Date:        item.date,
		Performers:  item.performers,
		Description: item.description,
		Tags:        item.tags,
		Thumbnail:   item.thumb,
		Duration:    item.duration,
		Studio:      "Pure CFNM",
		ScrapedAt:   now,
	}
}

var (
	blockRe     = regexp.MustCompile(`data-setid="(\d+)"`)
	titleRe     = regexp.MustCompile(`(?s)<a[^>]*href="/join/"[^>]*>\s*([^<]+?)\s*</a>`)
	performerRe = regexp.MustCompile(`href="[^"]*models/[^"]*">([^<]+)</a>`)
	dateValRe   = regexp.MustCompile(`([A-Z][a-z]+ \d{1,2}, \d{4})`)
	durationRe  = regexp.MustCompile(`(\d+)\s*(?:&nbsp;)*\s*minute`)
	thumbRe     = regexp.MustCompile(`class="update_thumb[^"]*"[^>]*src="([^"]+)"`)
	maxPageRe   = regexp.MustCompile(`_(\d+)_d\.html`)
)

func parseListingPage(body []byte, base string) []sceneItem {
	page := string(body)
	locs := blockRe.FindAllStringSubmatchIndex(page, -1)
	items := make([]sceneItem, 0, len(locs))

	for i, loc := range locs {
		id := page[loc[2]:loc[3]]
		start := loc[0]
		end := len(page)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		block := page[start:end]

		item := sceneItem{id: id}

		if m := titleRe.FindStringSubmatch(block); m != nil {
			item.title = strings.TrimSpace(html.UnescapeString(m[1]))
		}

		for _, m := range performerRe.FindAllStringSubmatch(block, -1) {
			name := strings.TrimSpace(html.UnescapeString(m[1]))
			if name != "" {
				item.performers = append(item.performers, name)
			}
		}

		if m := dateValRe.FindStringSubmatch(block); m != nil {
			if t, err := time.Parse("January 2, 2006", m[1]); err == nil {
				item.date = t.UTC()
			}
		}

		if m := durationRe.FindStringSubmatch(block); m != nil {
			mins, _ := strconv.Atoi(m[1])
			item.duration = mins * 60
		}

		if m := thumbRe.FindStringSubmatch(block); m != nil {
			thumb := m[1]
			if strings.HasPrefix(thumb, "/") {
				thumb = base + thumb
			}
			item.thumb = thumb
		}

		if item.title == "" {
			continue
		}

		items = append(items, item)
	}
	return items
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

var (
	modelBlockRe = regexp.MustCompile(`class="update_block"`)
	modelIDRe    = regexp.MustCompile(`id="set-target-(\d+)-`)
	modelTitleRe = regexp.MustCompile(`(?s)class="update_title"[^>]*>(.*?)</span>`)
	modelDateRe  = regexp.MustCompile(`(?s)class="update_date"[^>]*>\s*(.*?)\s*</span>`)
	modelDescRe  = regexp.MustCompile(`(?s)class="latest_update_description"[^>]*>(.*?)</span>`)
	modelTagsRe  = regexp.MustCompile(`(?s)class="tour_update_tags"[^>]*>(.*?)</span>`)
	tagLinkRe    = regexp.MustCompile(`>([^<]+)</a>`)
	modelThumbRe = regexp.MustCompile(`class="large_update_thumb[^"]*"[^>]*src="([^"]+)"`)
)

func parseModelPage(body []byte, base string) []sceneItem {
	page := string(body)
	locs := modelBlockRe.FindAllStringIndex(page, -1)
	var items []sceneItem

	for i, loc := range locs {
		start := loc[0]
		end := len(page)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		block := page[start:end]

		var item sceneItem

		if m := modelIDRe.FindStringSubmatch(block); m != nil {
			item.id = m[1]
		}
		if item.id == "" {
			continue
		}

		if m := modelTitleRe.FindStringSubmatch(block); m != nil {
			item.title = strings.TrimSpace(html.UnescapeString(m[1]))
		}

		for _, m := range performerRe.FindAllStringSubmatch(block, -1) {
			name := strings.TrimSpace(html.UnescapeString(m[1]))
			if name != "" {
				item.performers = append(item.performers, name)
			}
		}

		if m := modelDateRe.FindStringSubmatch(block); m != nil {
			raw := strings.TrimSpace(m[1])
			if t, err := time.Parse("January 2, 2006", raw); err == nil {
				item.date = t.UTC()
			}
		}

		if m := modelDescRe.FindStringSubmatch(block); m != nil {
			item.description = strings.TrimSpace(html.UnescapeString(m[1]))
		}

		if m := modelTagsRe.FindStringSubmatch(block); m != nil {
			for _, tm := range tagLinkRe.FindAllStringSubmatch(m[1], -1) {
				tag := strings.TrimSpace(html.UnescapeString(tm[1]))
				if tag != "" {
					item.tags = append(item.tags, tag)
				}
			}
		}

		if m := modelThumbRe.FindStringSubmatch(block); m != nil {
			thumb := m[1]
			if strings.HasPrefix(thumb, "/") {
				thumb = base + thumb
			}
			item.thumb = thumb
		}

		if m := durationRe.FindStringSubmatch(block); m != nil {
			mins, _ := strconv.Atoi(m[1])
			item.duration = mins * 60
		}

		items = append(items, item)
	}
	return items
}

// --- HTTP ---

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
