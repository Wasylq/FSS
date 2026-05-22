package bang

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

type Scraper struct {
	client *http.Client
}

func New() *Scraper {
	return &Scraper{
		client: httpx.NewClient(30 * time.Second),
	}
}

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return "bang" }

func (s *Scraper) Patterns() []string {
	return []string{
		"bang.com/originals/{id}/{slug}",
		"bang.com/originals",
		"bang.com/original/{id}/{slug}",
		"bang.com/studio/{id}/{slug}",
		"bang.com/pornstar/{id}/{slug}",
		"bang.com/videos?in={series}",
		"bang.com/videos?from={studio}",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?bang\.com/(?:originals?(?:/\d+/[\w-]+)?|studio/\d+/[\w-]+|pornstar/[\w-]+/[\w-]+|videos\?)`)

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	originalsRe = regexp.MustCompile(`/originals?/(\d+)/([\w-]+)`)
	studioRe    = regexp.MustCompile(`/studio/(\d+)/([\w-]+)`)
	pornstarRe  = regexp.MustCompile(`/pornstar/([\w-]+)/([\w-]+)`)
)

type urlMode int

const (
	modeOriginals urlMode = iota
	modeStudio
	modePornstar
	modeVideos
)

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	mode, err := detectMode(studioURL)
	if err != nil {
		select {
		case out <- scraper.Error(err):
		case <-ctx.Done():
		}
		return
	}

	scraper.Debugf(1, "bang: scraping %s", modeLabel(mode))

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

		pageURL := buildPageURL(studioURL, mode, page)
		scraper.Debugf(1, "bang: fetching page %d", page)

		items, total, hasNext, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		if len(items) == 0 {
			return
		}

		if page == 1 && total > 0 {
			scraper.Debugf(1, "bang: %d total scenes", total)
			select {
			case out <- scraper.Progress(total):
			case <-ctx.Done():
				return
			}
		}

		now := time.Now().UTC()
		for _, item := range items {
			if opts.KnownIDs[item.id] {
				scraper.Debugf(1, "bang: hit known ID, stopping early")
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
				return
			}
			scene := toScene(studioURL, item, now)
			select {
			case out <- scraper.Scene(scene):
			case <-ctx.Done():
				return
			}
		}

		if !hasNext {
			return
		}
	}
}

func detectMode(studioURL string) (urlMode, error) {
	u, err := url.Parse(studioURL)
	if err != nil {
		return 0, fmt.Errorf("invalid URL %q: %w", studioURL, err)
	}
	switch {
	case originalsRe.MatchString(u.Path):
		return modeOriginals, nil
	case studioRe.MatchString(u.Path):
		return modeStudio, nil
	case pornstarRe.MatchString(u.Path):
		return modePornstar, nil
	case u.Path == "/videos" || u.Path == "/videos/":
		q := u.Query()
		if q.Get("in") != "" || q.Get("from") != "" {
			return modeVideos, nil
		}
		return 0, fmt.Errorf("bang: /videos URL requires ?in= or ?from= parameter: %s", studioURL)
	case u.Path == "/originals" || u.Path == "/originals/":
		return 0, fmt.Errorf("bang: /originals hub page is not scrapable; use a specific originals URL like /originals/3366/bang-real-teens")
	default:
		return 0, fmt.Errorf("bang: unrecognized URL pattern: %s", studioURL)
	}
}

func modeLabel(m urlMode) string {
	switch m {
	case modeOriginals:
		return "originals series"
	case modeStudio:
		return "studio page"
	case modePornstar:
		return "pornstar page"
	case modeVideos:
		return "video filter"
	default:
		return "unknown"
	}
}

func buildPageURL(studioURL string, mode urlMode, page int) string {
	u, _ := url.Parse(studioURL)

	switch mode {
	case modeOriginals:
		m := originalsRe.FindStringSubmatch(u.Path)
		u.Path = fmt.Sprintf("/originals/%s/%s", m[1], m[2])
	case modeStudio:
		m := studioRe.FindStringSubmatch(u.Path)
		u.Path = fmt.Sprintf("/studio/%s/%s", m[1], m[2])
		u.Path = strings.TrimSuffix(u.Path, "/movies")
	case modePornstar:
		m := pornstarRe.FindStringSubmatch(u.Path)
		u.Path = fmt.Sprintf("/pornstar/%s/%s", m[1], m[2])
	}

	q := u.Query()
	q.Set("by", "date")
	q.Set("page", strconv.Itoa(page))
	u.RawQuery = q.Encode()
	return u.String()
}

// item holds parsed data from a single video card.
type item struct {
	id         string
	urlPath    string
	title      string
	thumbnail  string
	duration   int
	date       time.Time
	performers []string
	views      int
}

var (
	containerRe = regexp.MustCompile(`(?s)<div class="video_container[^"]*"[^>]*>.*?</div>\s*</div>\s*</div>`)
	idRe        = regexp.MustCompile(`data-videopreview-id-value="([a-f0-9]{24})"`)
	durationRe  = regexp.MustCompile(`data-videopreview-duration-value="(\d+)"`)
	hrefRe      = regexp.MustCompile(`href="(/video/[^"]+)"`)
	titleRe     = regexp.MustCompile(`(?s)<span class="block text-xs lg:text-sm text-foreground font-semibold truncate[^"]*">([^<]+)</span>`)
	thumbRe     = regexp.MustCompile(`<img[^>]+data-videopreview-target="image"[^>]+src="([^"]+)"`)
	dateRe      = regexp.MustCompile(`<span class="mx-1 lg:mx-2">.*?</span>\s*([A-Z][a-z]+ \d{1,2}, \d{4})`)
	viewsRe     = regexp.MustCompile(`(?s)<svg[^>]*viewBox="0 0 576 512"[^>]*>.*?</svg>\s*\n\s*(\d[\d.]*K?)`)
	performerRe = regexp.MustCompile(`<a[^>]+class="scrollup text-ring font-medium capitalize[^"]*"[^>]+href="/pornstar/[^"]*">([^<]+)</a>`)
	totalRe     = regexp.MustCompile(`(?s)<p[^>]+id="resultsCount"[^>]*>\s*([\d,]+)\s*results`)
	nextRe      = regexp.MustCompile(`<link rel="next"`)
)

func (s *Scraper) fetchPage(ctx context.Context, rawURL string) ([]item, int, bool, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     rawURL,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, 0, false, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return nil, 0, false, fmt.Errorf("reading page: %w", err)
	}

	total := 0
	if m := totalRe.FindSubmatch(body); m != nil {
		total, _ = strconv.Atoi(strings.ReplaceAll(string(m[1]), ",", ""))
	}

	hasNext := nextRe.Match(body)
	items := parseItems(body)
	return items, total, hasNext, nil
}

func parseItems(body []byte) []item {
	cards := containerRe.FindAll(body, -1)
	items := make([]item, 0, len(cards))
	for _, card := range cards {
		if it, ok := parseItem(card); ok {
			items = append(items, it)
		}
	}
	return items
}

func parseItem(card []byte) (item, bool) {
	m := idRe.FindSubmatch(card)
	if m == nil {
		return item{}, false
	}
	it := item{id: string(m[1])}

	if mHref := hrefRe.FindSubmatch(card); mHref != nil {
		it.urlPath = string(mHref[1])
	}

	if mTitle := titleRe.FindSubmatch(card); mTitle != nil {
		it.title = html.UnescapeString(strings.TrimSpace(string(mTitle[1])))
	}

	if mThumb := thumbRe.FindSubmatch(card); mThumb != nil {
		it.thumbnail = string(mThumb[1])
	}

	if mDur := durationRe.FindSubmatch(card); mDur != nil {
		it.duration, _ = strconv.Atoi(string(mDur[1]))
	}

	if mDate := dateRe.FindSubmatch(card); mDate != nil {
		if t, err := time.Parse("Jan 2, 2006", string(mDate[1])); err == nil {
			it.date = t
		}
	}

	if mViews := viewsRe.FindSubmatch(card); mViews != nil {
		it.views = parseViews(string(mViews[1]))
	}

	for _, mPerf := range performerRe.FindAllSubmatch(card, -1) {
		it.performers = append(it.performers, html.UnescapeString(strings.TrimSpace(string(mPerf[1]))))
	}

	return it, true
}

func parseViews(s string) int {
	s = strings.TrimSpace(s)
	multiplier := 1
	if strings.HasSuffix(s, "K") {
		multiplier = 1000
		s = strings.TrimSuffix(s, "K")
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int(f * float64(multiplier))
	}
	return 0
}

func toScene(studioURL string, it item, now time.Time) models.Scene {
	sceneURL := "https://www.bang.com" + it.urlPath

	return models.Scene{
		ID:         it.id,
		SiteID:     "bang",
		StudioURL:  studioURL,
		Title:      it.title,
		URL:        sceneURL,
		Thumbnail:  it.thumbnail,
		Duration:   it.duration,
		Date:       it.date,
		Performers: it.performers,
		Views:      it.views,
		ScrapedAt:  now,
	}
}
