package pornhub

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

func init() {
	scraper.Register(New())
}

func (s *Scraper) ID() string { return "pornhub" }

func (s *Scraper) Patterns() []string {
	return []string{
		"pornhub.com/pornstar/{slug}",
		"pornhub.com/channels/{slug}",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?pornhub\.com/(?:pornstar|channels)/[\w-]+`)
var pornstarRe = regexp.MustCompile(`/pornstar/([\w-]+)`)
var channelRe = regexp.MustCompile(`/channels/([\w-]+)`)

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// ---- runner ----

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

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

		pageURL, err := buildPageURL(studioURL, page)
		if err != nil {
			select {
			case out <- scraper.Error(err):
			case <-ctx.Done():
			}
			return
		}

		items, total, err := s.fetchPage(ctx, pageURL)
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
			select {
			case out <- scraper.Progress(total):
			case <-ctx.Done():
				return
			}
		}

		now := time.Now().UTC()
		for _, item := range items {
			if opts.KnownIDs[item.vkey] {
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
	}
}

// buildPageURL derives the paginated video-list URL from a studio URL.
// Pornstar: /pornstar/{slug}/videos?page=N
// Channel:  /channels/{slug}/videos?page=N
func buildPageURL(studioURL string, page int) (string, error) {
	u, err := url.Parse(studioURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL %q: %w", studioURL, err)
	}
	if m := pornstarRe.FindStringSubmatch(u.Path); m != nil {
		u.Path = "/pornstar/" + m[1] + "/videos"
		u.RawQuery = "page=" + strconv.Itoa(page)
		return u.String(), nil
	}
	if m := channelRe.FindStringSubmatch(u.Path); m != nil {
		u.Path = "/channels/" + m[1] + "/videos"
		u.RawQuery = "page=" + strconv.Itoa(page)
		return u.String(), nil
	}
	return "", fmt.Errorf("cannot extract pornhub slug from %q", studioURL)
}

// ---- page fetch ----

var liRe = regexp.MustCompile(`(?s)<li[^>]*pcVideoListItem[^>]*>.*?</li>`)

func (s *Scraper) fetchPage(ctx context.Context, rawURL string) ([]phItem, int, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: rawURL,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentFirefox,
			"Cookie":     "platform=pc; ageVerified=1; accessAgeDisclaimerPH=1",
		},
	})
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("reading page: %w", err)
	}

	total := 0
	if m := videoCountRe.FindSubmatch(body); m != nil {
		total, _ = strconv.Atoi(strings.ReplaceAll(string(m[1]), ",", ""))
	}

	return parseItems(body), total, nil
}

// ---- parsing ----

var (
	vkeyRe       = regexp.MustCompile(`data-video-vkey="([\w]+)"`)
	titleRe      = regexp.MustCompile(`href="/view_video\.php\?viewkey=[^"]*"\s+title="([^"]+)"`)
	thumbSrcRe   = regexp.MustCompile(`<img[^>]+src="(https://[^"]+)"`)
	durRe        = regexp.MustCompile(`<var[^>]*duration[^>]*>([^<]+)</var>`)
	cdnDateRe    = regexp.MustCompile(`/videos/(\d{4})(\d{2})/(\d{2})/`)
	uploaderRe   = regexp.MustCompile(`(?s)class="usernameWrap"[^>]*>.*?<a[^>]+>([^<]+)</a>`)
	videoCountRe = regexp.MustCompile(`showingCounter">\s*(\d[\d,]*)`)
)

type phItem struct {
	vkey      string
	title     string
	thumbnail string
	duration  int
	date      time.Time
	studio    string
}

func parseItems(body []byte) []phItem {
	lis := liRe.FindAll(body, -1)
	items := make([]phItem, 0, len(lis))
	for _, li := range lis {
		if item, ok := parseItem(li); ok {
			items = append(items, item)
		}
	}
	return items
}

func parseItem(li []byte) (phItem, bool) {
	m := vkeyRe.FindSubmatch(li)
	if m == nil {
		return phItem{}, false
	}
	item := phItem{vkey: string(m[1])}

	if mTitle := titleRe.FindSubmatch(li); mTitle != nil {
		item.title = html.UnescapeString(string(mTitle[1]))
	}

	if mThumb := thumbSrcRe.FindSubmatch(li); mThumb != nil {
		item.thumbnail = string(mThumb[1])
	}

	if mDate := cdnDateRe.FindStringSubmatch(item.thumbnail); mDate != nil {
		y, _ := strconv.Atoi(mDate[1])
		mo, _ := strconv.Atoi(mDate[2])
		d, _ := strconv.Atoi(mDate[3])
		item.date = time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC)
	}

	if mDur := durRe.FindSubmatch(li); mDur != nil {
		item.duration = parseDuration(strings.TrimSpace(string(mDur[1])))
	}

	if mStudio := uploaderRe.FindSubmatch(li); mStudio != nil {
		item.studio = strings.TrimSpace(string(mStudio[1]))
	}

	return item, true
}

func toScene(studioURL string, item phItem, now time.Time) models.Scene {
	scene := models.Scene{
		ID:        item.vkey,
		SiteID:    "pornhub",
		StudioURL: studioURL,
		Title:     item.title,
		URL:       "https://www.pornhub.com/view_video.php?viewkey=" + item.vkey,
		Thumbnail: item.thumbnail,
		Duration:  item.duration,
		Date:      item.date,
		Studio:    item.studio,
		ScrapedAt: now,
	}
	scene.AddPrice(models.PriceSnapshot{
		Date:   now,
		IsFree: true,
	})
	return scene
}

// parseDuration converts "MM:SS" or "HH:MM:SS" to seconds.
func parseDuration(s string) int {
	parts := strings.Split(s, ":")
	total := 0
	for _, p := range parts {
		n, _ := strconv.Atoi(p)
		total = total*60 + n
	}
	return total
}
