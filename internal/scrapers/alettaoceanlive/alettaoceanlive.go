// Package alettaoceanlive scrapes Aletta Ocean Live (alettaoceanlive.com), an
// Elevated X tour behind a custom Vue front-end skin (`movie-set-list-item`
// cards).
//
// It is a solo-performer site with no sibling brands. The catalogue is split
// across two categories, both walked here:
//
//   - `movies` — ~298 scenes with real `/tour/trailers/{slug}.html` pages.
//   - `homevideos` — ~106 clips whose cards link to `/join` rather than a
//     detail page, so the listing card is all there is.
//
// Everything usable is on the card: id, title, date and thumbnail. **The detail
// page adds nothing** — its body is an `<h1>` and a player, with no
// description, tags or runtime — so there is no detail worker pool. Duration is
// not published anywhere on the site.
//
// Performers are not marked up at all. The site is Aletta Ocean's own, so she
// is credited on every scene; guest performers appear only in titles and are
// not recoverable.
package alettaoceanlive

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const (
	siteID     = "alettaoceanlive"
	studioName = "Aletta Ocean Live"
	// Card dates are US-format.
	dateLayout = "01/02/2006"
)

var siteBase = "https://alettaoceanlive.com"

// categories are the two listings that together hold the whole catalogue.
var categories = []string{"movies", "homevideos"}

// Scraper implements scraper.StudioScraper for Aletta Ocean Live.
type Scraper struct {
	Client *http.Client
}

// New constructs an Aletta Ocean Live scraper.
func New() *Scraper {
	return &Scraper{Client: httpx.NewClient(30 * time.Second)}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		"alettaoceanlive.com",
		"alettaoceanlive.com/tour/categories/movies_{N}_d.html",
		"alettaoceanlive.com/tour/categories/homevideos_{N}_d.html",
		"alettaoceanlive.com/tour/trailers/{slug}.html",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?alettaoceanlive\.com(?:/|$)`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// ---- runner ----

// run walks the two categories in sequence. Paginate handles one page sequence
// at a time, so the category index advances as each listing runs dry.
func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	// Ids are unique across both categories (movies use content slugs,
	// homevideos an "H-" prefix), so one seen-set spans the whole run.
	seen := make(map[string]bool)
	category := 0
	// pageOffset maps Paginate's running page number onto the current
	// category's own numbering.
	pageOffset := 0

	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		for category < len(categories) {
			body, err := s.fetchPage(ctx, listingURL(categories[category], page-pageOffset))
			if err != nil {
				return scraper.PageResult{}, err
			}

			items := parseListing(body)
			fresh := items[:0]
			for _, it := range items {
				if !seen[it.id] {
					seen[it.id] = true
					fresh = append(fresh, it)
				}
			}
			if len(fresh) > 0 {
				return scraper.PageResult{Scenes: s.toScenes(studioURL, fresh, now)}, nil
			}

			// This category is exhausted; restart numbering for the next one.
			scraper.Debugf(1, "%s: category %q exhausted at page %d", siteID, categories[category], page-pageOffset)
			category++
			pageOffset = page - 1
		}
		return scraper.PageResult{Done: true}, nil
	})
}

// listingURL builds a category page. Page 1 has no numeric suffix.
func listingURL(category string, page int) string {
	if page <= 1 {
		return fmt.Sprintf("%s/tour/categories/%s.html", siteBase, category)
	}
	return fmt.Sprintf("%s/tour/categories/%s_%d_d.html", siteBase, category, page)
}

// ---- listing ----

var (
	// The class token must end right here: the card's own children are
	// `movie-set-list-item__wrapper`, `__content` and `__details`, so a looser
	// pattern splits every card into fragments and the first one — the only
	// place the id is read from — ends before the title and date.
	cardRe = regexp.MustCompile(`<div class="movie-set-list-item[ "]`)
	// The thumbnail is an inline background-image, not an <img>. Its content
	// directory is also the only stable id the site exposes.
	bgRe     = regexp.MustCompile(`background-image: url\(([^)]+)\)`)
	pathIDRe = regexp.MustCompile(`/tour/content/([^/]+)/`)
	linkRe   = regexp.MustCompile(`<a href="([^"]+)"`)
	titleRe  = regexp.MustCompile(`movie-set-list-item__title[^"]*">\s*([^<]*?)\s*</div>`)
	dateRe   = regexp.MustCompile(`movie-set-list-item__date[^"]*">\s*(\d{2}/\d{2}/\d{4})\s*</div>`)
)

type listItem struct {
	id, url, title, thumb string
	date                  time.Time
}

func parseListing(body []byte) []listItem {
	page := string(body)
	starts := cardRe.FindAllStringIndex(page, -1)
	items := make([]listItem, 0, len(starts))

	for i, loc := range starts {
		end := len(page)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		card := page[loc[0]:end]

		bg := bgRe.FindStringSubmatch(card)
		if bg == nil {
			continue
		}
		it := listItem{thumb: normalizeURL(strings.TrimSpace(bg[1]))}
		if m := pathIDRe.FindStringSubmatch(bg[1]); m != nil {
			it.id = m[1]
		}
		if it.id == "" {
			continue
		}

		if m := titleRe.FindStringSubmatch(card); m != nil {
			it.title = cleanText(m[1])
		}
		if m := dateRe.FindStringSubmatch(card); m != nil {
			if t, err := time.Parse(dateLayout, m[1]); err == nil {
				it.date = t.UTC()
			}
		}
		// Home-video cards link to /join instead of a detail page, so only a
		// real trailer link is kept as the scene URL.
		if m := linkRe.FindStringSubmatch(card); m != nil && strings.Contains(m[1], "/tour/trailers/") {
			it.url = normalizeURL(m[1])
		}

		items = append(items, it)
	}
	return items
}

func normalizeURL(u string) string {
	switch {
	case strings.HasPrefix(u, "//"):
		return "https:" + u
	case strings.HasPrefix(u, "/"):
		return siteBase + u
	case strings.HasPrefix(u, "http"):
		return u
	default:
		return siteBase + "/tour/" + u
	}
}

// ---- scenes ----

// toScenes builds scenes straight from the cards. The detail page carries only
// an <h1> and a player, so fetching it would add nothing.
func (s *Scraper) toScenes(studioURL string, items []listItem, now time.Time) []models.Scene {
	scenes := make([]models.Scene, 0, len(items))
	for _, it := range items {
		sceneURL := it.url
		if sceneURL == "" {
			// Home videos have no page of their own; point at the listing so
			// the scene still carries a resolvable URL.
			sceneURL = listingURL("homevideos", 1)
		}
		scenes = append(scenes, models.Scene{
			ID:        it.id,
			SiteID:    siteID,
			StudioURL: studioURL,
			Title:     it.title,
			URL:       sceneURL,
			Date:      it.date,
			Thumbnail: it.thumb,
			Studio:    studioName,
			// The site is Aletta Ocean's own and marks up no cast at all.
			Performers: []string{"Aletta Ocean"},
			ScrapedAt:  now,
		})
	}
	return scenes
}

func cleanText(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(s)), " ")
}

// ---- HTTP ----

func (s *Scraper) fetchPage(ctx context.Context, rawURL string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL:     rawURL,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}
