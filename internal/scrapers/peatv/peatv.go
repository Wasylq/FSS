package peatv

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const siteBase = "https://pea-tv.jp"

type Scraper struct {
	client *http.Client
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func New() *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second)}
}

func init() { scraper.Register(New()) }

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?pea-tv\.jp\b`)

func (s *Scraper) ID() string { return "peatv" }
func (s *Scraper) Patterns() []string {
	return []string{
		"pea-tv.jp/",
		"pea-tv.jp/search.php?b={brand}",
	}
}
func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	if opts.Workers <= 0 {
		opts.Workers = 3
	}
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

type listItem struct {
	code      string
	title     string
	thumbnail string
	duration  int
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	baseURL := buildListingURL(studioURL)
	scraper.Debugf(1, "peatv: listing base %s", baseURL)

	work := make(chan listItem, opts.Workers)
	var wg sync.WaitGroup
	for i := 0; i < opts.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range work {
				if opts.Delay > 0 {
					select {
					case <-time.After(opts.Delay):
					case <-ctx.Done():
						return
					}
				}
				scene, err := s.fetchDetail(ctx, item, studioURL)
				if err != nil {
					select {
					case out <- scraper.Error(fmt.Errorf("detail %s: %w", item.code, err)):
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

		u := pageURL(baseURL, page)
		scraper.Debugf(1, "peatv: fetching page %d", page)

		body, err := s.fetchHTML(ctx, u)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("listing page %d: %w", page, err)):
			case <-ctx.Done():
			}
			break
		}

		items, total, lastPage := parseListingPage(body)
		if len(items) == 0 {
			break
		}

		if page == 1 && total > 0 {
			scraper.Debugf(1, "peatv: %d total items", total)
			select {
			case out <- scraper.Progress(total):
			case <-ctx.Done():
				break
			}
		}

		hitKnown := false
		for _, item := range items {
			if opts.KnownIDs[item.code] {
				scraper.Debugf(1, "peatv: hit known ID %s, stopping early", item.code)
				hitKnown = true
				break
			}
			select {
			case work <- item:
			case <-ctx.Done():
				hitKnown = true
			}
			if ctx.Err() != nil {
				break
			}
		}
		if hitKnown {
			if opts.KnownIDs != nil {
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
			}
			break
		}

		if lastPage > 0 && page >= lastPage {
			break
		}
	}

	close(work)
	wg.Wait()
}

func buildListingURL(studioURL string) string {
	u, err := url.Parse(studioURL)
	if err != nil {
		return siteBase + "/search.php"
	}
	if strings.Contains(u.Path, "search.php") {
		q := u.Query()
		q.Del("p")
		base := siteBase + "/search.php"
		if encoded := q.Encode(); encoded != "" {
			base += "?" + encoded
		}
		return base
	}
	return siteBase + "/search.php"
}

func pageURL(base string, page int) string {
	if page <= 1 {
		return base
	}
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + "p=" + strconv.Itoa(page)
}

// ---- Listing page parsing ----

var (
	codeRe     = regexp.MustCompile(`monthly_detail\.php\?code=([^"&]+)`)
	altRe      = regexp.MustCompile(`alt="([^"]+)"`)
	thumbSrcRe = regexp.MustCompile(`src="\./?(pic_base/product/[^"]+)"`)
	durMinRe   = regexp.MustCompile(`■(\d+)分`)
	totalRe    = regexp.MustCompile(`(\d+)\s*件中`)
	lastPageRe = regexp.MustCompile(`href="[^"]*[?&](?:amp;)?p=(\d+)"[^>]*title="last page"`)
	cardSep    = []byte(`<div class="hori5">`)
)

func parseListingPage(body []byte) ([]listItem, int, int) {
	total := 0
	if m := totalRe.FindSubmatch(body); m != nil {
		total, _ = strconv.Atoi(string(m[1]))
	}
	lastPage := 0
	if m := lastPageRe.FindSubmatch(body); m != nil {
		lastPage, _ = strconv.Atoi(string(m[1]))
	}

	parts := bytes.Split(body, cardSep)
	if len(parts) <= 1 {
		return nil, total, lastPage
	}

	seen := make(map[string]bool)
	var items []listItem
	for _, part := range parts[1:] {
		m := codeRe.FindSubmatch(part)
		if m == nil {
			continue
		}
		code := string(m[1])
		if seen[code] {
			continue
		}
		seen[code] = true

		item := listItem{code: code}

		if m := altRe.FindSubmatch(part); m != nil {
			item.title = html.UnescapeString(string(m[1]))
		}
		if m := thumbSrcRe.FindSubmatch(part); m != nil {
			item.thumbnail = siteBase + "/" + string(m[1])
		}
		if m := durMinRe.FindSubmatch(part); m != nil {
			mins, _ := strconv.Atoi(string(m[1]))
			item.duration = mins * 60
		}

		items = append(items, item)
	}
	return items, total, lastPage
}

// ---- Detail page parsing ----

var (
	streamDateRe = regexp.MustCompile(`配信開始日</td><td[^>]*>([^<]+)</td>`)
	mailDateRe   = regexp.MustCompile(`通販開始日</td><td[^>]*>([^<]+)</td>`)
	jpDateRe     = regexp.MustCompile(`(\d+)年(\d+)月(\d+)日`)
	descRe       = regexp.MustCompile(`(?s)<p class="text-justify">(.*?)</p>`)
)

func (s *Scraper) fetchDetail(ctx context.Context, item listItem, studioURL string) (models.Scene, error) {
	now := time.Now().UTC()
	scene := models.Scene{
		ID:        item.code,
		SiteID:    "peatv",
		StudioURL: studioURL,
		Title:     item.title,
		URL:       siteBase + "/monthly_detail.php?code=" + item.code,
		Thumbnail: item.thumbnail,
		Duration:  item.duration,
		Studio:    "PEA-TV",
		ScrapedAt: now,
	}

	body, err := s.fetchHTML(ctx, scene.URL)
	if err != nil {
		return scene, err
	}

	if d, ok := parseDetailDate(body); ok {
		scene.Date = d
	}

	if m := descRe.FindSubmatch(body); m != nil {
		desc := strings.TrimSpace(html.UnescapeString(string(m[1])))
		desc = strings.ReplaceAll(desc, "<br>", " ")
		desc = strings.ReplaceAll(desc, "<br/>", " ")
		desc = strings.ReplaceAll(desc, "<br />", " ")
		scene.Description = strings.TrimSpace(desc)
	}

	return scene, nil
}

func parseDetailDate(body []byte) (time.Time, bool) {
	if m := streamDateRe.FindSubmatch(body); m != nil {
		if d, ok := parseJPDate(string(m[1])); ok {
			return d, true
		}
	}
	if m := mailDateRe.FindSubmatch(body); m != nil {
		if d, ok := parseJPDate(string(m[1])); ok {
			return d, true
		}
	}
	return time.Time{}, false
}

func parseJPDate(s string) (time.Time, bool) {
	m := jpDateRe.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, false
	}
	y, _ := strconv.Atoi(m[1])
	mo, _ := strconv.Atoi(m[2])
	d, _ := strconv.Atoi(m[3])
	if y == 0 || mo == 0 || d == 0 {
		return time.Time{}, false
	}
	return time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC), true
}

func (s *Scraper) fetchHTML(ctx context.Context, rawURL string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: rawURL,
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
