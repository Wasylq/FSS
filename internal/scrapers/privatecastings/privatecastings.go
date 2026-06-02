// Package privatecastings scrapes privatecastings.com — the Private
// network's casting-couch sub-brand. Same CMS family as privateblack.com
// (Bootstrap-grid Private CMS variant) but the listing card is stripped
// further: no `<span class="scene-date">`, no preview video, and the
// thumbnail ships only via a single `<img src>` (not a `<picture><source
// srcset>` block).
//
// Card markup:
//
//	<li class="col-lg-3 col-md-4 col-sm-6 col-xs-12">
//	  <div class="scene">
//	    <a href="https://www.privatecastings.com/scene/{slug}/{id}"
//	       id="vthumb_{id}" class="scene-thumb" title="…">
//	      <img src="https://pcastings77.st-content.com/…/contentthumbs/{n}.jpg?secure=…"
//	           title="…" class="img-responsive" />
//	    </a>
//	    <ul class="scene-votes"><li><span/> <a href="…/scene/…">38</a></li></ul>
//	    <ul class="scene-models">
//	      <li><a href="https://www.privatecastings.com/pornstar/{n}-{slug}/">Name</a></li>
//	    </ul>
//	    <h3><a href="https://www.privatecastings.com/scene/{slug}/{id}">Title</a></h3>
//	  </div>
//	</li>
//
// Pagination: `/scenes/{N}/` (path-based, trailing slash required).
// Past-end pages return zero `<div class="scene">` cards.
//
// Date is not present in the listing markup; `Scene.Date` stays zero.
// Listing is still date-sorted newest-first so `KnownIDs` early-stop
// works — we just can't surface the date itself without a detail-page
// round trip, which the scraper deliberately avoids for speed.
package privatecastings

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
	defaultBase = "https://www.privatecastings.com"
	scraperID   = "privatecastings"
	studioName  = "Private"
	seriesName  = "Private Castings"
)

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

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return scraperID }
func (s *Scraper) Patterns() []string {
	return []string{
		"privatecastings.com/",
		"privatecastings.com/scenes",
		"privatecastings.com/scenes/{N}/",
		"privatecastings.com/pornstar/{id}-{slug}/",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?privatecastings\.com(?:/|$)`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	cardStartRe = regexp.MustCompile(`<div class="scene">`)
	sceneURLRe  = regexp.MustCompile(
		`href="(https?://(?:www\.)?privatecastings\.com/scene/[^"]+/(\d+))"`,
	)
	titleH3Re   = regexp.MustCompile(`(?s)<h3>\s*<a[^>]+href="[^"]+/scene/[^"]+/\d+"[^>]*>\s*([^<]+?)\s*</a>\s*</h3>`)
	performerRe = regexp.MustCompile(
		`<a[^>]+href="[^"]*/pornstar/\d+-[^"]+"[^>]*>\s*([^<]+?)\s*</a>`,
	)
	imgSrcRe   = regexp.MustCompile(`<img[^>]+src="(https?://pcastings77\.st-content\.com/[^"]+)"`)
	pageLinkRe = regexp.MustCompile(`href="[^"]*/scenes/(\d+)/?"`)
)

type sceneItem struct {
	id         string
	url        string
	title      string
	performers []string
	thumb      string
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
		if m := sceneURLRe.FindStringSubmatch(block); m != nil {
			item.url = m[1]
			item.id = m[2]
		}
		if item.id == "" || seen[item.id] {
			continue
		}
		seen[item.id] = true

		if m := titleH3Re.FindStringSubmatch(block); m != nil {
			item.title = html.UnescapeString(strings.TrimSpace(m[1]))
		}
		for _, pm := range performerRe.FindAllStringSubmatch(block, -1) {
			name := html.UnescapeString(strings.TrimSpace(pm[1]))
			if name != "" {
				item.performers = append(item.performers, name)
			}
		}
		item.performers = dedupStrings(item.performers)
		if m := imgSrcRe.FindStringSubmatch(block); m != nil {
			item.thumb = m[1]
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
		return s.base + "/scenes"
	}
	return fmt.Sprintf("%s/scenes/%d/", s.base, page)
}

var pornstarPathRe = regexp.MustCompile(`/pornstar/(\d+-[A-Za-z0-9-]+)`)

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	if pornstarPathRe.MatchString(studioURL) {
		scraper.Debugf(1, "privatecastings: detected pornstar page")
		s.scrapePornstarPage(ctx, studioURL, opts, out)
		return
	}

	scraper.Debugf(1, "privatecastings: scraping full catalog")
	now := time.Now().UTC()
	firstPage := true
	scraper.Paginate(ctx, opts, "privatecastings", out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := s.listingURL(page)
		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}
		items := parseListing(body)
		var total int
		if firstPage {
			total = estimateTotal(body, len(items))
			firstPage = false
		}
		scenes := make([]models.Scene, len(items))
		for i, item := range items {
			scenes[i] = s.toScene(item, studioURL, now)
		}
		return scraper.PageResult{Scenes: scenes, Total: total}, nil
	})
}

func (s *Scraper) scrapePornstarPage(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	pageURL := studioURL
	if !strings.HasPrefix(pageURL, "http") {
		pageURL = s.base + pageURL
	}

	body, err := s.fetchPage(ctx, pageURL)
	if err != nil {
		select {
		case out <- scraper.Error(err):
		case <-ctx.Done():
		}
		return
	}

	items := parseListing(body)
	if len(items) == 0 {
		return
	}
	scraper.Debugf(1, "privatecastings: found %d scenes on pornstar page", len(items))

	now := time.Now().UTC()
	select {
	case out <- scraper.Progress(len(items)):
	case <-ctx.Done():
		return
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
		case out <- scraper.Scene(s.toScene(item, studioURL, now)):
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scraper) toScene(item sceneItem, studioURL string, now time.Time) models.Scene {
	return models.Scene{
		ID:         item.id,
		SiteID:     scraperID,
		StudioURL:  studioURL,
		Title:      item.title,
		URL:        item.url,
		Thumbnail:  item.thumb,
		Performers: item.performers,
		Studio:     studioName,
		Series:     seriesName,
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
