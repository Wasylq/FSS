// Package claudiamarie scrapes Claudia Marie (claudiamarie.com), an Elevated X
// classic tour.
//
// It shares the `update_block` skin with glosstightsglamour, but differs enough
// to stay standalone:
//
//   - There are no per-scene detail pages at all. Every in-card link goes to
//     join.php, so the listing is the complete public dataset and scene URLs
//     are synthesised from the content slug.
//   - The truncated `latest_update_description` is superseded by the full text
//     carried in the `title=` attribute of the trailer anchor.
//   - Each page also renders a "coming soon" sidebar with future-dated entries
//     that share the `update_date` class, so every field is read from inside an
//     `update_block` rather than page-wide.
//
// The content slug encodes a date that is not the release date (040726… vs a
// published 07/15/2026), so only `update_date` is trusted.
package claudiamarie

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
	siteID     = "claudiamarie"
	studioName = "Claudia Marie"
)

var siteBase = "https://claudiamarie.com"

// Scraper implements scraper.StudioScraper for Claudia Marie.
type Scraper struct {
	Client *http.Client
}

// New constructs a Claudia Marie scraper.
func New() *Scraper {
	return &Scraper{Client: httpx.NewClient(30 * time.Second)}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		"claudiamarie.com",
		"claudiamarie.com/tour/updates/page_{N}.html",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?claudiamarie\.com`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// ---- runner ----

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	seen := make(map[string]bool)
	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := fmt.Sprintf("%s/tour/updates/page_%d.html", siteBase, page)
		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}

		items := parseListing(body)
		// Pages past the last one return HTTP 200 with an empty template, so
		// end-of-listing is detected by zero cards rather than by status.
		if len(items) == 0 {
			return scraper.PageResult{Done: true}, nil
		}

		scenes := make([]models.Scene, 0, len(items))
		for _, it := range items {
			if seen[it.id] {
				continue
			}
			seen[it.id] = true
			scenes = append(scenes, it.toScene(studioURL, now))
		}
		return scraper.PageResult{Scenes: scenes, Continue: len(scenes) == 0}, nil
	})
}

// ---- listing ----

var (
	// Every field is read from inside an update_block: the page also renders a
	// future-dated "coming soon" sidebar that reuses the update_date class.
	blockRe = regexp.MustCompile(`<div class="update_block">`)
	// The content slug is the only stable id — there is no numeric one.
	slugRe    = regexp.MustCompile(`tload\('/trailers/([A-Za-z0-9_-]+)\.mp4'\)`)
	altSlugRe = regexp.MustCompile(`src="content/([A-Za-z0-9_-]+)/\d+\.jpg"`)
	titleRe   = regexp.MustCompile(`<h2 class="update_title">([^<]+)</h2>`)
	dateRe    = regexp.MustCompile(`<span class="update_date">(\d{2}/\d{2}/\d{4})</span>`)
	// The visible description is truncated with a trailing "..."; the anchor's
	// title attribute holds the full text.
	fullDescRe  = regexp.MustCompile(`title="([^"]{40,})"\s*alt=`)
	shortDescRe = regexp.MustCompile(`<span class="latest_update_description">([^<]*)</span>`)
	modelsRe    = regexp.MustCompile(`(?s)<span class="tour_update_models">(.*?)</span>`)
	modelLinkRe = regexp.MustCompile(`/models/[^"]*\.html">([^<]+)</a>`)
	tagsRe      = regexp.MustCompile(`(?s)<span class="tour_update_tags">(.*?)</span>`)
	tagLinkRe   = regexp.MustCompile(`/categories/[^"]*\.html">([^<]+)</a>`)
	thumbRe     = regexp.MustCompile(`src="(content/[A-Za-z0-9_-]+/\d+\.jpg)"`)
	// "30&nbsp;Photos, 21&nbsp;minute(s)&nbsp;of video"
	durationRe = regexp.MustCompile(`(\d+)&nbsp;minute\(s\)`)
)

type listItem struct {
	id, title, description, thumb string
	date                          time.Time
	duration                      int
	performers, tags              []string
}

func parseListing(body []byte) []listItem {
	page := string(body)
	starts := blockRe.FindAllStringIndex(page, -1)
	items := make([]listItem, 0, len(starts))

	for i, loc := range starts {
		end := len(page)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		block := page[loc[0]:end]

		var it listItem
		if m := slugRe.FindStringSubmatch(block); m != nil {
			it.id = m[1]
		} else if m := altSlugRe.FindStringSubmatch(block); m != nil {
			it.id = m[1]
		}
		if it.id == "" {
			continue
		}

		if m := titleRe.FindStringSubmatch(block); m != nil {
			it.title = html.UnescapeString(strings.TrimSpace(m[1]))
		}
		if m := dateRe.FindStringSubmatch(block); m != nil {
			// US-format date. The slug also encodes a date, but it is the
			// content-folder date, not the release date.
			if ts, err := time.Parse("01/02/2006", m[1]); err == nil {
				it.date = ts.UTC()
			}
		}
		if m := fullDescRe.FindStringSubmatch(block); m != nil {
			it.description = cleanText(m[1])
		} else if m := shortDescRe.FindStringSubmatch(block); m != nil {
			it.description = cleanText(m[1])
		}
		if mb := modelsRe.FindStringSubmatch(block); mb != nil {
			it.performers = collect(modelLinkRe, mb[1])
		}
		if tb := tagsRe.FindStringSubmatch(block); tb != nil {
			it.tags = collect(tagLinkRe, tb[1])
		}
		if m := thumbRe.FindStringSubmatch(block); m != nil {
			it.thumb = siteBase + "/tour/" + m[1]
		}
		if m := durationRe.FindStringSubmatch(block); m != nil {
			it.duration = atoi(m[1]) * 60
		}

		items = append(items, it)
	}
	return items
}

func (it listItem) toScene(studioURL string, now time.Time) models.Scene {
	return models.Scene{
		ID:        it.id,
		SiteID:    siteID,
		StudioURL: studioURL,
		Title:     it.title,
		// There are no per-scene pages; every card links to join.php. The URL
		// is synthesised so each scene still has a stable anchor.
		URL:         fmt.Sprintf("%s/tour/#%s", siteBase, it.id),
		Date:        it.date,
		Description: it.description,
		Thumbnail:   it.thumb,
		Duration:    it.duration,
		Performers:  it.performers,
		Tags:        it.tags,
		Studio:      studioName,
		ScrapedAt:   now,
	}
}

func collect(re *regexp.Regexp, s string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		v := html.UnescapeString(strings.TrimSpace(m[1]))
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func cleanText(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(s)), " ")
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return n
		}
		n = n*10 + int(r-'0')
	}
	return n
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
