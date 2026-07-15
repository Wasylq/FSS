// Package scissorgoddess scrapes Scissor Goddess (scissorgoddess.net), the
// home of Goddess Rapture's Fetish Playground.
//
// The site is WordPress + WooCommerce: scenes are `product` posts, not `post`
// posts (the standard posts endpoint holds a single "Hello world!" stub). The
// WP REST API is open and `_embed` resolves every taxonomy in one request, so
// there is no detail fetch and no second round-trip to name terms:
//
//	/wp-json/wp/v2/product?per_page=100&page=N&_embed
//
// Taxonomies map as: `model` -> performers, `genre` -> categories,
// `product_tag` -> tags. `product_cat` is the storefront section ("Video") and
// `product_brand` the publisher, neither of which is scene metadata.
package scissorgoddess

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
	siteID     = "scissorgoddess"
	studioName = "Goddess Rapture's Fetish Playground"
	perPage    = 100
)

var siteBase = "https://scissorgoddess.net"

// Scraper implements scraper.StudioScraper for Scissor Goddess.
type Scraper struct {
	Client *http.Client
}

// New constructs a Scissor Goddess scraper.
func New() *Scraper {
	return &Scraper{Client: httpx.NewClient(30 * time.Second)}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		"scissorgoddess.net",
		"scissorgoddess.net/product/{slug}/",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?scissorgoddess\.net(?:/|$)`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// ---- API types ----

type wpProduct struct {
	ID       int        `json:"id"`
	DateGMT  string     `json:"date_gmt"`
	Link     string     `json:"link"`
	Title    wpRendered `json:"title"`
	Content  wpRendered `json:"content"`
	Embedded struct {
		FeaturedMedia []struct {
			SourceURL string `json:"source_url"`
		} `json:"wp:featuredmedia"`
		Terms [][]wpTerm `json:"wp:term"`
	} `json:"_embedded"`
}

type wpRendered struct {
	Rendered string `json:"rendered"`
}

type wpTerm struct {
	Name     string `json:"name"`
	Taxonomy string `json:"taxonomy"`
}

// ---- runner ----

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	totalPages := 0
	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		products, total, pages, err := s.fetchPage(ctx, page)
		if err != nil {
			// WP answers HTTP 400 for a page past the last one. Past page 1
			// that is the end of the listing, not a failure.
			if page > 1 {
				scraper.Debugf(1, "%s: page %d past end (%v), stopping", siteID, page, err)
				return scraper.PageResult{Done: true}, nil
			}
			return scraper.PageResult{}, err
		}
		if pages > 0 {
			totalPages = pages
		}

		scenes := make([]models.Scene, 0, len(products))
		for _, p := range products {
			scenes = append(scenes, toScene(studioURL, p, now))
		}
		return scraper.PageResult{
			Scenes: scenes,
			Total:  total,
			Done:   (totalPages > 0 && page >= totalPages) || len(products) < perPage,
		}, nil
	})
}

func (s *Scraper) fetchPage(ctx context.Context, page int) ([]wpProduct, int, int, error) {
	u := fmt.Sprintf("%s/wp-json/wp/v2/product?per_page=%d&page=%d&_embed", siteBase, perPage, page)
	scraper.Debugf(1, "%s: fetching page %d", siteID, page)

	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL:     u,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, 0, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	total, _ := strconv.Atoi(resp.Header.Get("X-WP-Total"))
	totalPages, _ := strconv.Atoi(resp.Header.Get("X-WP-TotalPages"))

	var products []wpProduct
	if err := httpx.DecodeJSON(resp.Body, &products); err != nil {
		return nil, 0, 0, fmt.Errorf("decoding page %d: %w", page, err)
	}
	return products, total, totalPages, nil
}

// ---- scene conversion ----

var tagStripRe = regexp.MustCompile(`<[^>]+>`)

func toScene(studioURL string, p wpProduct, now time.Time) models.Scene {
	scene := models.Scene{
		ID:        strconv.Itoa(p.ID),
		SiteID:    siteID,
		StudioURL: studioURL,
		Title:     cleanText(p.Title.Rendered),
		URL:       p.Link,
		Studio:    studioName,
		ScrapedAt: now,
	}

	if p.DateGMT != "" {
		if t, err := time.Parse("2006-01-02T15:04:05", p.DateGMT); err == nil {
			scene.Date = t.UTC()
		}
	}
	scene.Description = cleanText(tagStripRe.ReplaceAllString(p.Content.Rendered, " "))

	if m := p.Embedded.FeaturedMedia; len(m) > 0 {
		scene.Thumbnail = m[0].SourceURL
	}

	// _embed returns one slice per taxonomy; only three of them are scene
	// metadata. product_cat is the storefront section and product_brand the
	// publisher, so both are ignored.
	for _, group := range p.Embedded.Terms {
		for _, term := range group {
			name := cleanText(term.Name)
			if name == "" {
				continue
			}
			switch term.Taxonomy {
			case "model":
				scene.Performers = append(scene.Performers, name)
			case "genre":
				scene.Categories = append(scene.Categories, name)
			case "product_tag":
				scene.Tags = append(scene.Tags, name)
			}
		}
	}

	return scene
}

// cleanText unescapes HTML entities and collapses whitespace. WP renders
// typographic entities (&#038;, &#8211;) into titles and descriptions.
func cleanText(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(s)), " ")
}
