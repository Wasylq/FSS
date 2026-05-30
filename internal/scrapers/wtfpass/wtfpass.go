// Package wtfpass scrapes the WTFPass network — a Russian softcore /
// amateur group with a shared catalogue served by wtfpass.com and 16
// sister-brand domains, all running the same custom CMS. Every card
// labels its source sub-site via `<span class="site"><a>{Site}</a></span>`,
// so the parent pass automatically tags each scene with its true brand on
// `Scene.Series`. Sister-domain scrapes get the same template with the
// site filtered to one brand.
//
// Card markup (one card):
//
//	<div class="thumb-video cf">
//	  <div class="thumb-container">
//	    <a class="thumb-video-link" href="https://wtfpass.com/videos/{id}/{slug}/" title="{title}">
//	      <img class="thumb" src="https://wtfpass.com/contents/videos_screenshots/{X}/{id}/360x240/1.jpg" alt="{title}" />
//	      <span class="rating">…87%</span>
//	      <span class="duration">27 min</span>
//	      <span class="hd">HD</span>
//	    </a>
//	  </div>
//	  <div class="thumb-data">
//	    <p class="title"><a href="…">{title}</a></p>
//	    <p class="data-row site-and-date">
//	      <span class="site"><a href="/sites/{slug}/">{Site Display Name}</a></span>
//	    </p>
//	    <p class="data-row data-categories">
//	      <a class="link-blue" href="/categories/{slug}/">{category}</a> …
//	    </p>
//	  </div>
//	  <div class="thumb-data-extend">
//	    <div class="video-data">
//	      <div class="data-row site-and-date">
//	        <span class="site"><a>{Site}</a></span>
//	        <span class="date-added">12 years ago</span>
//	        <span class="views">964 709</span>
//	      </div>
//	    </div>
//	  </div>
//	</div>
//
// Fields lifted: ID + slug from the detail URL, title (from the
// `title=` attribute on the thumb anchor — the title text is also
// available as the `<p class="title">` anchor text), MM:min duration
// (`<span class="duration">27 min</span>` → 27*60 sec), thumb, source
// sub-site (`<span class="site">`), categories list, and views count.
//
// Pagination: `/videos/{N}/` with trailing slash; max page is in
// `<div class="pagination">` with a `<a href="/videos/{N}/">{N}</a>`
// near a `<span class="button unactive">…</span>` separator. Past-end
// pages return zero cards (clean stop signal).
//
// Detail pages are public on the parent — the scene URL we lift from
// `<a class="thumb-video-link" href="…">` works as-is for downstream
// matching.
//
// `<span class="date-added">12 years ago</span>` is a relative date string
// that we don't parse for now; `Scene.Date` stays zero on this CMS.
// Stash matching uses title + performer + filename so the missing date
// doesn't materially hurt downstream identification.
package wtfpass

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

const studioName = "WTFPass"

// SiteConfig describes one WTFPass network site. SiteBase has no trailing
// slash. SiteName is the human-readable label used as a fallback when a
// card doesn't carry its own `<span class="site">` (which happens on
// listings filtered to one brand — the card just omits the redundant
// label). The parent uses SiteName="" so the per-card label always wins.
type SiteConfig struct {
	ID       string
	SiteBase string
	SiteName string
	Patterns []string
	MatchRe  *regexp.Regexp
}

type Scraper struct {
	cfg    SiteConfig
	client *http.Client
}

func New(cfg SiteConfig) *Scraper {
	return &Scraper{
		cfg:    cfg,
		client: httpx.NewClient(30 * time.Second),
	}
}

func (s *Scraper) ID() string         { return s.cfg.ID }
func (s *Scraper) Patterns() []string { return s.cfg.Patterns }
func (s *Scraper) MatchesURL(u string) bool {
	return s.cfg.MatchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	cardStartRe = regexp.MustCompile(`<div class="thumb-video cf">`)
	// Detail URL — `<a class="thumb-video-link" href="…/videos/{id}/{slug}/" title="…">`.
	// Captures the URL + the numeric ID.
	detailURLRe = regexp.MustCompile(
		`<a class="thumb-video-link"\s+href="(https?://[^"]+/videos/(\d+)/[^"]+/?)"\s+title="([^"]*)"`,
	)
	// Duration — `<span class="duration">27 min</span>` (no padding/MM:SS;
	// always minutes).
	durationRe = regexp.MustCompile(`<span class="duration\s*">\s*(\d+)\s*min\s*</span>`)
	// Thumbnail — `<img class="thumb" src="…">`.
	thumbRe = regexp.MustCompile(`<img class="thumb"[^>]+src="([^"]+)"`)
	// Source sub-site — `<span class="site">…<a … class="link-gray">{name}</a></span>`.
	siteSpanRe = regexp.MustCompile(
		`(?s)<span class="site">.*?<a[^>]+class="link-gray"[^>]*>\s*([^<]+?)\s*</a>`,
	)
	// Categories — `<a class="link-blue" href="…/categories/…/">{name}</a>`.
	categoryRe = regexp.MustCompile(
		`<a[^>]+class="link-blue"[^>]+href="[^"]*/categories/[^"]+"[^>]*>\s*([^<]+?)\s*</a>`,
	)
	// Views count — `<span class="views">…964 709</span>`.
	viewsRe = regexp.MustCompile(
		`(?s)<span class="views">.*?</span>\s*([\d\s]+?)\s*</span>`,
	)
	// Pagination max page — every page link in `<div class="pagination">`
	// matches `/videos/{N}/`; we take the highest.
	pageLinkRe = regexp.MustCompile(`href="[^"]*/videos/(\d+)/"`)
)

type sceneItem struct {
	id         string
	title      string
	url        string
	thumb      string
	duration   int // seconds
	series     string
	categories []string
	views      int
}

func parseListing(body []byte) []sceneItem {
	page := string(body)
	starts := cardStartRe.FindAllStringIndex(page, -1)
	items := make([]sceneItem, 0, len(starts))
	seen := make(map[string]bool, len(starts))

	for i, loc := range starts {
		end := len(page)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		block := page[loc[0]:end]

		var item sceneItem
		if m := detailURLRe.FindStringSubmatch(block); m != nil {
			item.url = m[1]
			item.id = m[2]
			item.title = html.UnescapeString(strings.TrimSpace(m[3]))
		}
		if item.id == "" || seen[item.id] {
			continue
		}
		seen[item.id] = true

		if m := thumbRe.FindStringSubmatch(block); m != nil {
			item.thumb = m[1]
		}
		if m := durationRe.FindStringSubmatch(block); m != nil {
			mins, _ := strconv.Atoi(m[1])
			item.duration = mins * 60
		}
		if m := siteSpanRe.FindStringSubmatch(block); m != nil {
			item.series = html.UnescapeString(strings.TrimSpace(m[1]))
		}
		for _, cm := range categoryRe.FindAllStringSubmatch(block, -1) {
			cat := html.UnescapeString(strings.TrimSpace(cm[1]))
			if cat != "" {
				item.categories = append(item.categories, cat)
			}
		}
		item.categories = dedupStrings(item.categories)
		if m := viewsRe.FindStringSubmatch(block); m != nil {
			// "964 709" → 964709.
			raw := strings.ReplaceAll(m[1], " ", "")
			raw = strings.ReplaceAll(raw, ",", "")
			if n, err := strconv.Atoi(raw); err == nil {
				item.views = n
			}
		}

		items = append(items, item)
	}
	return items
}

func estimateTotal(body []byte, perPage int) int {
	maxPage := 1
	for _, m := range pageLinkRe.FindAllSubmatch(body, -1) {
		n, _ := strconv.Atoi(string(m[1]))
		if n > maxPage {
			maxPage = n
		}
	}
	return maxPage * perPage
}

func (s *Scraper) listingURL(page int) string {
	if page <= 1 {
		return s.cfg.SiteBase + "/videos/"
	}
	return fmt.Sprintf("%s/videos/%d/", s.cfg.SiteBase, page)
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)
	scraper.Debugf(1, "%s: scraping catalog", s.cfg.ID)

	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, s.cfg.ID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := s.listingURL(page)

		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}

		items := parseListing(body)

		total := 0
		if page == 1 {
			total = estimateTotal(body, len(items))
		}

		scenes := make([]models.Scene, len(items))
		for i, item := range items {
			scenes[i] = s.toScene(item, studioURL, now)
		}

		return scraper.PageResult{
			Scenes: scenes,
			Total:  total,
		}, nil
	})
}

func (s *Scraper) toScene(item sceneItem, studioURL string, now time.Time) models.Scene {
	series := item.series
	if series == "" {
		series = s.cfg.SiteName
	}
	return models.Scene{
		ID:         item.id,
		SiteID:     s.cfg.ID,
		StudioURL:  studioURL,
		Title:      item.title,
		URL:        item.url,
		Thumbnail:  item.thumb,
		Duration:   item.duration,
		Studio:     studioName,
		Series:     series,
		Categories: item.categories,
		Views:      item.views,
		ScrapedAt:  now,
	}
}

func (s *Scraper) fetchPage(ctx context.Context, url string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     url,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}

func dedupStrings(in []string) []string {
	if len(in) <= 1 {
		return in
	}
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
