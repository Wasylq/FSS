// Package romeromultimedia scrapes the Romero Multimedia network — a
// fetish/horror-themed studio whose sister sites all run WordPress and
// expose the standard WP REST API. Each sister site is registered as its
// own scraper ID; the shared parser walks `/wp-json/wp/v2/posts?_embed`
// pages and lifts the rich payload (title, description, date, thumbnail,
// performers via `post_tag`, categories, director) onto models.Scene.
//
// The lone exception is **Twinz**, which never got its own domain — its
// catalogue lives behind `hentaied.pro/wp-json/wp/v2/posts?origin_website=411`
// (the `origin_website` taxonomy on the membership portal identifies which
// sub-site a post belongs to). The same shared parser handles it; the
// SiteConfig just points at hentaied.pro with `OriginWebsiteID=411`.
//
// Out-of-scope (would need its own scraper / not currently working):
//
//   - footfetish.center — `/wp-json/wp/v2/posts` returns HTTP 404 (likely
//     a different CMS or the WP REST API has been disabled).
//
// WP REST API contract (verified against all sister sites):
//
//   - `/wp-json/wp/v2/posts?per_page=100&_embed&page=N` — paginated listing
//   - `X-WP-Total` and `X-WP-TotalPages` headers — pagination metadata
//   - `_embed` adds `_embedded.wp:featuredmedia[]` (thumb) and
//     `_embedded.wp:term[][]` (categories, tags, directors, origin_website)
//   - `title.rendered`, `content.rendered`, `excerpt.rendered` — HTML
//     fields that we flatten via html.UnescapeString + tag-strip
//   - Sort: WP default is `?orderby=date&order=desc` (newest first), so
//     `KnownIDs` early-stop works without specifying it explicitly.
//
// Listing-only is fine here: the WP REST payload already carries everything
// stash needs. No detail-page round trip required.
package romeromultimedia

import (
	"context"
	"encoding/json"
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

const (
	studioName       = "Romero Multimedia"
	wpPerPage        = 100
	wpPostsPathFmt   = "/wp-json/wp/v2/posts?per_page=%d&_embed=1&page=%d"
	wpPostsPathFiltF = "/wp-json/wp/v2/posts?per_page=%d&_embed=1&origin_website=%d&page=%d"
)

// SiteConfig describes one Romero Multimedia sister site. SiteBase has no
// trailing slash. When OriginWebsiteID is non-zero, posts are filtered by
// the `origin_website` taxonomy term — used for Twinz (which lives only on
// the hentaied.pro portal).
type SiteConfig struct {
	ID              string
	SiteBase        string
	SiteName        string // human-readable display name (Scene.Series)
	OriginWebsiteID int    // 0 = none; otherwise filter posts by this term
	Patterns        []string
	MatchRe         *regexp.Regexp
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

// wpPost is the slice of `/wp-json/wp/v2/posts?_embed` fields we use. The
// real payload has ~30 fields, most of which we ignore.
type wpPost struct {
	ID       int      `json:"id"`
	DateGMT  string   `json:"date_gmt"`
	Slug     string   `json:"slug"`
	Link     string   `json:"link"`
	Title    rendered `json:"title"`
	Content  rendered `json:"content"`
	Excerpt  rendered `json:"excerpt"`
	Embedded embedded `json:"_embedded"`
}

type rendered struct {
	Rendered string `json:"rendered"`
}

type embedded struct {
	FeaturedMedia []featuredMedia `json:"wp:featuredmedia,omitempty"`
	// wp:term is a list-of-lists keyed by taxonomy. Each inner list contains
	// the term records for that taxonomy. The order maps to the taxonomies
	// the embedded controller surfaces (typically [categories, tags,
	// directors, origin_website]) but we don't depend on the order — we
	// route each inner list by its own `taxonomy` field.
	Terms [][]term `json:"wp:term,omitempty"`
}

type featuredMedia struct {
	SourceURL string `json:"source_url"`
}

type term struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Taxonomy string `json:"taxonomy"`
}

func (s *Scraper) listingURL(page int) string {
	if s.cfg.OriginWebsiteID != 0 {
		return s.cfg.SiteBase + fmt.Sprintf(wpPostsPathFiltF, wpPerPage, s.cfg.OriginWebsiteID, page)
	}
	return s.cfg.SiteBase + fmt.Sprintf(wpPostsPathFmt, wpPerPage, page)
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)
	scraper.Debugf(1, "%s: scraping WP REST catalog", s.cfg.ID)

	now := time.Now().UTC()
	sentTotal := false
	totalPages := 0

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

		body, headers, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		var posts []wpPost
		if err := json.Unmarshal(body, &posts); err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: parse: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}
		if len(posts) == 0 {
			return
		}

		if !sentTotal {
			total, _ := strconv.Atoi(headers.Get("X-WP-Total"))
			totalPages, _ = strconv.Atoi(headers.Get("X-WP-TotalPages"))
			scraper.Debugf(1, "%s: %d total scenes across %d pages", s.cfg.ID, total, totalPages)
			if total > 0 {
				select {
				case out <- scraper.Progress(total):
				case <-ctx.Done():
					return
				}
			}
			sentTotal = true
		}

		for _, post := range posts {
			id := strconv.Itoa(post.ID)
			if opts.KnownIDs[id] {
				scraper.Debugf(1, "%s: hit known ID %s, stopping early", s.cfg.ID, id)
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
				return
			}
			select {
			case out <- scraper.Scene(s.toScene(post, studioURL, now)):
			case <-ctx.Done():
				return
			}
		}

		if totalPages > 0 && page >= totalPages {
			return
		}
	}
}

func (s *Scraper) toScene(p wpPost, studioURL string, now time.Time) models.Scene {
	id := strconv.Itoa(p.ID)
	sceneURL := p.Link
	if sceneURL == "" {
		// Fall back to a synthesised URL on the cfg base.
		sceneURL = s.cfg.SiteBase + "/?p=" + id
	}

	var (
		performers []string
		categories []string
		director   string
		thumb      string
	)
	if len(p.Embedded.FeaturedMedia) > 0 {
		thumb = p.Embedded.FeaturedMedia[0].SourceURL
	}
	for _, group := range p.Embedded.Terms {
		for _, t := range group {
			name := strings.TrimSpace(t.Name)
			if name == "" {
				continue
			}
			switch t.Taxonomy {
			case "post_tag":
				performers = append(performers, name)
			case "category":
				categories = append(categories, name)
			case "directors":
				if director == "" {
					director = name
				}
			}
		}
	}

	return models.Scene{
		ID:          id,
		SiteID:      s.cfg.ID,
		StudioURL:   studioURL,
		Title:       cleanHTML(p.Title.Rendered),
		URL:         sceneURL,
		Description: cleanHTML(p.Content.Rendered),
		Thumbnail:   thumb,
		Date:        parseWPDate(p.DateGMT),
		Performers:  performers,
		Categories:  categories,
		Director:    director,
		Studio:      studioName,
		Series:      s.cfg.SiteName,
		ScrapedAt:   now,
	}
}

func (s *Scraper) fetchPage(ctx context.Context, u string) ([]byte, http.Header, error) {
	parsed, err := url.Parse(u)
	if err != nil {
		return nil, nil, fmt.Errorf("parse url: %w", err)
	}
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: parsed.String(),
		Headers: map[string]string{
			"Accept":     "application/json",
			"User-Agent": httpx.UserAgentFirefox,
		},
	})
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	return body, resp.Header, nil
}

// parseWPDate parses WordPress' ISO-ish date format. The REST API serves
// `date_gmt` as `"2026-05-25T17:11:54"` (no timezone suffix — implicit UTC).
// Returns zero time on parse failure so the field is just empty rather
// than corrupting the scene.
func parseWPDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		"2006-01-02T15:04:05",
		time.RFC3339,
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

var (
	htmlTagRe = regexp.MustCompile(`<[^>]+>`)
	wsRe      = regexp.MustCompile(`\s+`)
)

// cleanHTML strips HTML tags from a string, decodes entities, and
// collapses whitespace. WP `*.rendered` fields commonly contain `<p>`,
// `<strong>`, `<a>`, and `&nbsp;`/`&amp;`/`&hellip;` entities that we
// flatten so downstream consumers see plain text.
func cleanHTML(s string) string {
	if s == "" {
		return ""
	}
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, " ", " ")
	s = wsRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
