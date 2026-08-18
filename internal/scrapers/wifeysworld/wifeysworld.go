// Package wifeysworld scrapes Wifey's World (wifeysworld.com).
//
// The site was rebuilt in 2026 and the NATS tour this scraper used to read is
// gone: `/v3/tour/categories/updates_{N}_d.html` now 404s, and the front page is
// a one-screen landing behind an age gate. What remains public is the store at
// `/store/`, which sells the catalogue as individual downloads — so a "scene"
// here is a store product, and it carries a price, which the tour never did.
//
// Two details worth knowing. The store's three categories are not equivalent:
// `wifey_movies` is the catalogue, while `wifey_wear` (clothing) and
// `wifey_trinkets` (commissioned custom shoots) are merchandise and services,
// so only movies are scraped unless the operator names a category explicitly.
// And the listing is one page for the whole category with no pagination, so a
// stored product is skipped rather than stopping the walk — the page is already
// fetched by then and stopping would only hide the products behind it.
package wifeysworld

import (
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

	"github.com/Anastylosis/FSS/internal/httpx"
	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/scraper"
)

const (
	siteID = "wifeysworld"
	studio = "Wifey's World"
	// moviesCategory is the store category holding the video catalogue.
	moviesCategory = "wifey_movies"
)

// baseURL is a var so tests can point it at an httptest server.
var baseURL = "https://wifeysworld.com"

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?wifeysworld\.com(?:/|$)`)

type Scraper struct {
	Client *http.Client
}

func New() *Scraper {
	return &Scraper{Client: httpx.NewClient(30 * time.Second)}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }
func (s *Scraper) Patterns() []string {
	return []string{
		"wifeysworld.com",
		"wifeysworld.com/store/",
		"wifeysworld.com/store/?category={slug}",
		"wifeysworld.com/store/product.php?slug={slug}",
	}
}
func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	cardRe      = regexp.MustCompile(`(?s)<div class="d1-card">(.*?)</div>\s*</div>\s*</div>`)
	cardSlugRe  = regexp.MustCompile(`/store/product\.php\?slug=([^"&]+)`)
	cardTitleRe = regexp.MustCompile(`(?s)<div class="d1-card-title"><a [^>]*>(.*?)</a>`)
	cardImgRe   = regexp.MustCompile(`<img src="([^"]+)"`)
	cardPriceRe = regexp.MustCompile(`<span class="d1-price">\s*\$([0-9.,]+)`)

	detailTitleRe = regexp.MustCompile(`(?s)<h1 class="d1-pd-title">(.*?)</h1>`)
	detailPriceRe = regexp.MustCompile(`<span class="d1-pd-price">\s*\$([0-9.,]+)`)
	detailDescRe  = regexp.MustCompile(`(?s)<div class="d1-pd-desc">(.*?)</div>`)
	detailImgRe   = regexp.MustCompile(`<img id="pdMain" src="([^"]+)"`)
	detailTypeRe  = regexp.MustCompile(`(?s)<div class="d1-pd-type">(.*?)</div>`)
	detailSKURe   = regexp.MustCompile(`(?s)<strong>SKU:</strong>\s*([^<]+)<`)

	productURLRe = regexp.MustCompile(`/store/product\.php\?slug=([^&?#]+)`)
	tagStripRe   = regexp.MustCompile(`<[^>]*>`)
)

// listingURL turns what the operator pointed at into the category listing to
// walk. A bare host means the video catalogue.
func listingURL(studioURL string) string {
	cat := moviesCategory
	if u, err := url.Parse(studioURL); err == nil {
		if c := u.Query().Get("category"); c != "" {
			cat = c
		}
	}
	return fmt.Sprintf("%s/store/?category=%s&sort=newest", baseURL, cat)
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()

	// A single product URL is a legitimate thing to point at.
	if m := productURLRe.FindStringSubmatch(studioURL); m != nil {
		scraper.Debugf(1, "%s: scraping one product %s", siteID, m[1])
		if !send(ctx, out, scraper.Progress(1)) {
			return
		}
		sc, err := s.fetchProduct(ctx, storeCard{slug: m[1]}, studioURL, now)
		if err != nil {
			send(ctx, out, scraper.Error(err))
			return
		}
		send(ctx, out, scraper.Scene(sc))
		return
	}

	pageURL := listingURL(studioURL)
	scraper.Debugf(1, "%s: fetching store listing %s", siteID, pageURL)
	body, err := s.fetchPage(ctx, pageURL)
	if err != nil {
		send(ctx, out, scraper.Error(err))
		return
	}

	cards := parseListing(body)
	if len(cards) == 0 {
		// A store that parses to nothing is a redesign, not an empty shop —
		// reporting it keeps an authoritative --full from deleting the
		// catalogue on the strength of a broken parser.
		send(ctx, out, scraper.Error(scraper.ParseError(pageURL,
			fmt.Errorf("no product cards in the store listing"))))
		return
	}
	scraper.Debugf(1, "%s: %d products", siteID, len(cards))

	// The listing is one page for the whole category, so a stored product is
	// skipped rather than ending the walk; only its detail fetch is saved.
	fresh := make([]storeCard, 0, len(cards))
	for _, c := range cards {
		if opts.KnownIDs[c.slug] {
			continue
		}
		fresh = append(fresh, c)
	}
	if skipped := len(cards) - len(fresh); skipped > 0 {
		scraper.Debugf(1, "%s: skipped %d already-stored product(s)", siteID, skipped)
		if !send(ctx, out, scraper.StoppedEarly()) {
			return
		}
	}
	if !send(ctx, out, scraper.Progress(len(cards))) {
		return
	}

	s.fetchAll(ctx, fresh, studioURL, opts, now, out)
}

func (s *Scraper) fetchAll(ctx context.Context, cards []storeCard, studioURL string, opts scraper.ListOpts, now time.Time, out chan<- scraper.SceneResult) {
	workers := opts.Workers
	if workers <= 0 {
		workers = 4
	}

	work := make(chan storeCard)
	var wg sync.WaitGroup
	// LIFO: close(work) ends the workers' range loops, then wg.Wait blocks
	// until they are gone, so a ctx.Done bail below cannot leak them.
	defer wg.Wait()
	defer close(work)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range work {
				if !sleep(ctx, opts.Delay) {
					return
				}
				sc, err := s.fetchProduct(ctx, c, studioURL, now)
				if err != nil {
					if !send(ctx, out, scraper.Error(err)) {
						return
					}
					continue
				}
				if !send(ctx, out, scraper.Scene(sc)) {
					return
				}
			}
		}()
	}

	for _, c := range cards {
		select {
		case work <- c:
		case <-ctx.Done():
			return
		}
	}
}

type storeCard struct {
	slug  string
	title string
	thumb string
	price float64
}

func parseListing(body []byte) []storeCard {
	page := string(body)
	seen := make(map[string]bool)
	var cards []storeCard

	for _, m := range cardRe.FindAllStringSubmatch(page, -1) {
		block := m[1]
		sm := cardSlugRe.FindStringSubmatch(block)
		if sm == nil || seen[sm[1]] {
			continue
		}
		seen[sm[1]] = true

		c := storeCard{slug: sm[1]}
		if t := cardTitleRe.FindStringSubmatch(block); t != nil {
			c.title = cleanText(t[1])
		}
		if i := cardImgRe.FindStringSubmatch(block); i != nil {
			c.thumb = absolute(i[1])
		}
		if p := cardPriceRe.FindStringSubmatch(block); p != nil {
			c.price = parsePrice(p[1])
		}
		cards = append(cards, c)
	}
	return cards
}

func (s *Scraper) fetchProduct(ctx context.Context, c storeCard, studioURL string, now time.Time) (models.Scene, error) {
	productURL := fmt.Sprintf("%s/store/product.php?slug=%s", baseURL, c.slug)

	scene := models.Scene{
		ID:        c.slug,
		SiteID:    siteID,
		StudioURL: studioURL,
		Title:     c.title,
		URL:       productURL,
		Studio:    studio,
		Thumbnail: c.thumb,
		ScrapedAt: now,
	}

	body, err := s.fetchPage(ctx, productURL)
	if err != nil {
		return models.Scene{}, fmt.Errorf("product %s: %w", c.slug, err)
	}
	page := string(body)

	if t := detailTitleRe.FindStringSubmatch(page); t != nil {
		if title := cleanText(t[1]); title != "" {
			scene.Title = title
		}
	}
	if scene.Title == "" {
		return models.Scene{}, scraper.ParseError(productURL, fmt.Errorf("no product title"))
	}
	if d := detailDescRe.FindStringSubmatch(page); d != nil {
		scene.Description = cleanText(d[1])
	}
	if i := detailImgRe.FindStringSubmatch(page); i != nil {
		scene.Thumbnail = absolute(i[1])
	}
	if ty := detailTypeRe.FindStringSubmatch(page); ty != nil {
		for _, part := range strings.Split(cleanText(ty[1]), "·") {
			if v := cleanText(part); v != "" {
				scene.Categories = append(scene.Categories, v)
			}
		}
	}
	if sk := detailSKURe.FindStringSubmatch(page); sk != nil {
		scene.ExternalIDs = map[string]string{"wifeysworld_sku": cleanText(sk[1])}
	}

	price := c.price
	if p := detailPriceRe.FindStringSubmatch(page); p != nil {
		price = parsePrice(p[1])
	}
	if price > 0 {
		scene.AddPrice(models.PriceSnapshot{Date: now, Regular: price})
	}
	return scene, nil
}

// ---- helpers ----

func parsePrice(s string) float64 {
	v, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64)
	if err != nil {
		return 0
	}
	return v
}

func absolute(u string) string {
	if u == "" || strings.HasPrefix(u, "http") {
		return u
	}
	return baseURL + "/" + strings.TrimPrefix(u, "/")
}

func cleanText(s string) string {
	s = tagStripRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, " ", " ")
	return strings.Join(strings.Fields(s), " ")
}

func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	select {
	case <-time.After(d):
		return true
	case <-ctx.Done():
		return false
	}
}

func send(ctx context.Context, out chan<- scraper.SceneResult, r scraper.SceneResult) bool {
	select {
	case out <- r:
		return true
	case <-ctx.Done():
		return false
	}
}

func (s *Scraper) fetchPage(ctx context.Context, url string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL:     url,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}
