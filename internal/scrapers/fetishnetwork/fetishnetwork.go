package fetishnetwork

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
	"github.com/Wasylq/FSS/parseutil"
	"github.com/Wasylq/FSS/scraper"
)

const (
	siteBase   = "https://www.fetishnetwork.com"
	defaultCat = "1765" // "all videos" paginated listing
)

var (
	matchRe = regexp.MustCompile(`^https?://(?:www\.)?fetishnetwork\.com(?:/|$)`)

	cardStartRe = regexp.MustCompile(`<!-- start_link -->`)
	sceneIDRe   = regexp.MustCompile(`refstat\.php\?lid=(\d+)&sid=\d+`)
	titleRe     = regexp.MustCompile(`(?s)class="text-infos img-name">\s*<a[^>]+>([^<]+)</a>`)
	thumbRe     = regexp.MustCompile(`<img[^>]+src="(faceimages/[^"]+)"`)
	dateRe      = regexp.MustCompile(`class="text-infos date-info">([^<]+)<`)
	siteNameRe  = regexp.MustCompile(`(?s)sub-content-left">\s*<p class="text-infos (?:type-of-sex|website-name)">\s*<a[^>]+>([^<]+)</a>`)
	categoryRe  = regexp.MustCompile(`(?s)sub-content-right">\s*<p class="text-infos type-of-sex">\s*<a[^>]+>([^<]+)</a>`)
	maxPageRe   = regexp.MustCompile(`a=\d+_(\d+)`)

	catIDRe = regexp.MustCompile(`[?&]a=(\d+)`)
)

const cardEnd = "<!-- end_link -->"

type Scraper struct {
	client *http.Client
}

func New() *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second)}
}

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return "fetishnetwork" }
func (s *Scraper) Patterns() []string {
	return []string{
		"fetishnetwork.com",
		"fetishnetwork.com/t2/show.php?a={catID}",
	}
}
func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

func baseURL(studioURL string) string {
	if u, err := url.Parse(studioURL); err == nil && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}
	return siteBase
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	catID := defaultCat
	if m := catIDRe.FindStringSubmatch(studioURL); m != nil {
		catID = m[1]
	}

	base := baseURL(studioURL)
	now := time.Now().UTC()

	scraper.Paginate(ctx, opts, "fetishnetwork", out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := fmt.Sprintf("%s/t2/show.php?a=%s_%d", base, catID, page)
		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}

		items := parseListingPage(body)
		if len(items) == 0 {
			return scraper.PageResult{}, nil
		}

		total := 0
		if page == 1 {
			total = estimateTotal(body, len(items))
		}

		scenes := make([]models.Scene, len(items))
		for i, item := range items {
			scenes[i] = item.toScene(studioURL, now)
		}
		return scraper.PageResult{Scenes: scenes, Total: total}, nil
	})
}

type sceneItem struct {
	id       string
	title    string
	thumb    string
	date     time.Time
	subSite  string
	category string
}

func parseListingPage(body []byte) []sceneItem {
	page := string(body)
	starts := cardStartRe.FindAllStringIndex(page, -1)
	seen := make(map[string]bool, len(starts))
	items := make([]sceneItem, 0, len(starts))

	for _, loc := range starts {
		rest := page[loc[0]:]
		endIdx := strings.Index(rest, cardEnd)
		if endIdx < 0 {
			continue
		}
		block := rest[:endIdx]

		var item sceneItem

		if m := sceneIDRe.FindStringSubmatch(block); m != nil {
			item.id = m[1]
		}
		if item.id == "" || seen[item.id] {
			continue
		}
		seen[item.id] = true

		if m := titleRe.FindStringSubmatch(block); m != nil {
			item.title = html.UnescapeString(strings.TrimSpace(m[1]))
		}

		if m := thumbRe.FindStringSubmatch(block); m != nil {
			item.thumb = siteBase + "/t2/" + m[1]
		}

		if m := dateRe.FindStringSubmatch(block); m != nil {
			item.date = parseDate(strings.TrimSpace(m[1]))
		}

		if m := siteNameRe.FindStringSubmatch(block); m != nil {
			item.subSite = strings.TrimSpace(m[1])
		}

		if m := categoryRe.FindStringSubmatch(block); m != nil {
			item.category = strings.TrimSpace(m[1])
		}

		items = append(items, item)
	}
	return items
}

func parseDate(s string) time.Time {
	t, _ := parseutil.TryParseDate(strings.TrimSpace(s), "January 2, 2006", "January 2 2006")
	return t
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

func (item sceneItem) toScene(studioURL string, now time.Time) models.Scene {
	sc := models.Scene{
		ID:        item.id,
		SiteID:    "fetishnetwork",
		StudioURL: studioURL,
		Title:     item.title,
		URL:       fmt.Sprintf("%s/t2/refstat.php?lid=%s&sid=%s", siteBase, item.id, defaultCat),
		Thumbnail: item.thumb,
		Date:      item.date,
		Studio:    "Fetish Network",
		ScrapedAt: now,
	}
	if item.subSite != "" {
		sc.Series = strings.TrimSuffix(item.subSite, ".com")
	}
	if item.category != "" {
		sc.Tags = []string{item.category}
	}
	return sc
}

func (s *Scraper) fetchPage(ctx context.Context, url string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     url,
		Headers: httpx.BrowserHeaders(httpx.UserAgentChrome),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}
