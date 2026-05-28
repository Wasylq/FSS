// Package darkreachupdatesutil scrapes Darkreach Communications performer
// sites running the "updates clear" affiliate-marketing template
// (clubkayden, thelisaann, theavaaddams, kortneykane).
//
// Card markup:
//
//	<div class="updates clear">
//	  <div class="model">
//	    <a href="https://join.{site}.com/signup/signup.php?nats=…">
//	      <img src="/content/{slug}/0.jpg" />
//	    </a>
//	  </div>
//	  <div class="modelDetails">
//	    <h3><a href="…signup.php?nats=…">Scene Title</a></h3>
//	    <div class="date"></div>
//	    <p>Description text… <a href="…">join now</a></p>
//	  </div>
//	</div>
//
// All scene anchors are affiliate `/signup.php?nats=...` URLs — there are no
// public detail pages. Scene IDs are synthesised from the thumbnail path
// (`/content/{slug}/0.jpg` → slug used as ID); URLs as `{base}/#scene-{id}`.
//
// Pagination: `/updates/page_{N}.html` (page 1 is `/` or `/updates/`).
package darkreachupdatesutil

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

type SiteConfig struct {
	ID       string
	SiteBase string
	Studio   string
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
	cardStartRe = regexp.MustCompile(`<div class="updates clear">`)
	// Thumb path doubles as the scene ID source — `/content/{slug}/0.jpg`.
	thumbRe = regexp.MustCompile(`<img\s+src="(/?content/([A-Za-z0-9_-]+)/[^"]+)"`)
	titleH3 = regexp.MustCompile(`(?s)<h3>\s*<a[^>]*>\s*([^<]+?)\s*</a>`)
	// Description paragraph (first <p> in the card before pagination markup).
	descRe = regexp.MustCompile(`(?s)<p>(.*?)</p>`)
	// Pagination URLs `/updates/page_N.html`.
	pageLinkRe = regexp.MustCompile(`/updates/page_(\d+)\.html`)
)

type sceneItem struct {
	id          string
	title       string
	thumb       string
	description string
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
		if m := thumbRe.FindStringSubmatch(block); m != nil {
			item.thumb = m[1]
			item.id = m[2]
		}
		if item.id == "" || seen[item.id] {
			continue
		}
		seen[item.id] = true

		if m := titleH3.FindStringSubmatch(block); m != nil {
			item.title = html.UnescapeString(strings.TrimSpace(m[1]))
		}
		if m := descRe.FindStringSubmatch(block); m != nil {
			// Strip inline anchors (e.g. "join now") from the description text.
			text := regexp.MustCompile(`<[^>]+>`).ReplaceAllString(m[1], "")
			text = strings.TrimSpace(html.UnescapeString(text))
			// Many entries end with a "join now" call-to-action — drop it.
			text = strings.TrimSuffix(text, "join now")
			text = strings.TrimSpace(text)
			item.description = text
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
		// Page 1 lives at the bare root — `/updates/page_1.html` typically
		// redirects there too.
		return s.cfg.SiteBase + "/"
	}
	return fmt.Sprintf("%s/updates/page_%d.html", s.cfg.SiteBase, page)
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)
	scraper.Debugf(1, "%s: scraping full catalog", s.cfg.ID)

	now := time.Now().UTC()
	sentTotal := false

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

		pageURL := s.listingURL(page)
		scraper.Debugf(1, "%s: fetching page %d", s.cfg.ID, page)

		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		items := parseListing(body)
		if len(items) == 0 {
			return
		}

		if !sentTotal {
			total := estimateTotal(body, len(items))
			if total > 0 {
				select {
				case out <- scraper.Progress(total):
				case <-ctx.Done():
					return
				}
			}
			sentTotal = true
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
			case out <- scraper.Scene(item.toScene(s.cfg.ID, s.cfg.SiteBase, s.cfg.Studio, now)):
			case <-ctx.Done():
				return
			}
		}
	}
}

func (item sceneItem) toScene(siteID, siteBase, studio string, now time.Time) models.Scene {
	thumb := item.thumb
	if thumb != "" && !strings.HasPrefix(thumb, "http") {
		thumb = siteBase + "/" + strings.TrimLeft(thumb, "/")
	}
	return models.Scene{
		ID:          item.id,
		SiteID:      siteID,
		StudioURL:   siteBase,
		Title:       item.title,
		URL:         fmt.Sprintf("%s/#scene-%s", siteBase, item.id),
		Thumbnail:   thumb,
		Description: item.description,
		Studio:      studio,
		ScrapedAt:   now,
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
