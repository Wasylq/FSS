// Package youthlust scrapes youthlust.club, a Shopify storefront that sells each
// scene as a product. The public Shopify products.json endpoint exposes the full
// catalogue (title, date, description, price, tags, thumbnail) with no auth.
package youthlust

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
	siteID   = "youthlust"
	studio   = "YouthLust"
	base     = "https://youthlust.club"
	pageSize = 250
)

type Scraper struct {
	client  *http.Client
	apiBase string // products.json host; overridable in tests
}

func New() *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second), apiBase: base}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{"youthlust.club", "youthlust.club/products/{handle}"}
}

func (s *Scraper) MatchesURL(u string) bool {
	return strings.Contains(u, "youthlust.club")
}

func (s *Scraper) ListScenes(ctx context.Context, _ string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, opts, out)
	return out, nil
}

func (s *Scraper) run(ctx context.Context, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		products, err := s.fetchPage(ctx, page)
		if err != nil {
			return scraper.PageResult{}, err
		}
		// Empty page = past the end of the catalogue.
		if len(products) == 0 {
			return scraper.PageResult{Done: true}, nil
		}

		var scenes []models.Scene
		for _, p := range products {
			// Only "Videos" products are scenes; skip bundles, alternate
			// versions, merch, etc.
			if !strings.EqualFold(p.ProductType, "Videos") {
				continue
			}
			scenes = append(scenes, toScene(p, now))
		}

		// A full page of non-video products must not stop pagination — keep
		// walking until Shopify returns an empty page.
		return scraper.PageResult{Scenes: scenes, Done: len(products) < pageSize, Continue: len(scenes) == 0}, nil
	})
}

func (s *Scraper) fetchPage(ctx context.Context, page int) ([]product, error) {
	u := fmt.Sprintf("%s/products.json?limit=%d&page=%d", s.apiBase, pageSize, page)
	scraper.Debugf(1, "%s: fetching page %d", siteID, page)
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     u,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var body productsResponse
	if err := httpx.DecodeJSON(resp.Body, &body); err != nil {
		return nil, fmt.Errorf("decoding products page %d: %w", page, err)
	}
	return body.Products, nil
}

func toScene(p product, now time.Time) models.Scene {
	sc := models.Scene{
		ID:          fmt.Sprintf("%d", p.ID),
		SiteID:      siteID,
		StudioURL:   base,
		Title:       strings.TrimSpace(p.Title),
		URL:         fmt.Sprintf("%s/products/%s", base, p.Handle),
		Description: stripHTML(p.BodyHTML),
		Studio:      studio,
		Tags:        p.Tags,
		ScrapedAt:   now,
	}
	if t := parseShopifyTime(p.PublishedAt); !t.IsZero() {
		sc.Date = t
	} else if t := parseShopifyTime(p.CreatedAt); !t.IsZero() {
		sc.Date = t
	}
	if len(p.Images) > 0 {
		sc.Thumbnail = p.Images[0].Src
	}
	if len(p.Variants) > 0 {
		if price, ok := parsePrice(p.Variants[0].Price); ok {
			date := sc.Date
			if date.IsZero() {
				date = now
			}
			sc.AddPrice(models.PriceSnapshot{Date: date, Regular: price})
		}
	}
	return sc
}

// parsePrice parses a Shopify decimal price string ("150.00") to a float.
func parsePrice(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}

// stripHTML removes tags from a body_html fragment and unescapes entities,
// collapsing whitespace into a single-spaced description.
func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

// parseShopifyTime parses Shopify's RFC3339 timestamps (e.g.
// "2026-06-25T20:52:34-06:00") to UTC.
func parseShopifyTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

type productsResponse struct {
	Products []product `json:"products"`
}

type product struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Handle      string    `json:"handle"`
	BodyHTML    string    `json:"body_html"`
	PublishedAt string    `json:"published_at"`
	CreatedAt   string    `json:"created_at"`
	Vendor      string    `json:"vendor"`
	ProductType string    `json:"product_type"`
	Tags        []string  `json:"tags"`
	Variants    []variant `json:"variants"`
	Images      []image   `json:"images"`
}

type variant struct {
	Price string `json:"price"`
}

type image struct {
	Src string `json:"src"`
}
