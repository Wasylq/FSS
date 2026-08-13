package thelisaannvod

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
	"sync"
	"time"

	"github.com/Anastylosis/FSS/internal/httpx"
	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/scraper"
)

// The VOD store is a second, larger catalogue on the same host as the
// `thelisaann` marketing listing in the darkreach package: 319 scenes with real
// detail pages and rent/buy pricing, against that page's 120 with synthesised
// `#scene-` URLs and no prices. It is a separate SiteID because the two number
// their scenes differently — the tour uses the CMS `data-setid`, the marketing
// page a title slug — so no merge of the two is possible and one scraper cannot
// serve both.
const (
	siteID      = "thelisaannvod"
	studioName  = "Lisa Ann"
	defaultBase = "https://thelisaann.com"
	tourPrefix  = "/vod"
)

type Scraper struct {
	Client       *http.Client
	baseOverride string
}

func New() *Scraper {
	return &Scraper{Client: httpx.NewClient(30 * time.Second)}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		"thelisaann.com/vod/",
		"thelisaann.com/vod/categories/movies_{N}_d.html",
		"thelisaann.com/vod/models/{slug}.html",
	}
}

// Scoped to /vod/ on purpose: the bare host belongs to the `thelisaann`
// marketing scraper, and both answering for it would make which one runs an
// accident of registration order.
var matchRe = regexp.MustCompile(`^https?://(?:www\.)?thelisaann\.com/vod(?:/|$)`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	if opts.Workers <= 0 {
		opts.Workers = 4
	}
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

func (s *Scraper) base(studioURL string) string {
	if s.baseOverride != "" {
		return strings.TrimSuffix(s.baseOverride, "/")
	}
	if u, err := url.Parse(studioURL); err == nil && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}
	return defaultBase
}

var modelURLRe = regexp.MustCompile(`/vod/models/([^/?#]+)\.html`)

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	base := s.base(studioURL)

	// A model page is the same listing filtered by the CMS, so it is walked
	// with the same pager — only the path stem differs.
	stem := "categories/movies"
	if m := modelURLRe.FindStringSubmatch(studioURL); m != nil && m[1] != "models" {
		stem = "models/" + m[1]
		scraper.Debugf(1, "%s: scraping model %q", siteID, m[1])
	} else {
		scraper.Debugf(1, "%s: scraping the full VOD catalogue", siteID)
	}

	cards, stopped := s.walk(ctx, base, stem, opts, out)
	if len(cards) == 0 {
		if stopped {
			send(ctx, out, scraper.StoppedEarly())
		}
		return
	}

	if !send(ctx, out, scraper.Progress(len(cards))) {
		return
	}
	scraper.Debugf(1, "%s: %d scenes, fetching details with %d workers", siteID, len(cards), opts.Workers)
	s.fetchDetails(ctx, studioURL, base, cards, opts, out)

	if stopped {
		send(ctx, out, scraper.StoppedEarly())
	}
}

const (
	// maxPages bounds the walk. The catalogue ends cleanly (page 15 serves no
	// cards), but a CMS that starts clamping out-of-range pages instead would
	// otherwise loop forever.
	maxPages = 80
)

func (s *Scraper) walk(ctx context.Context, base, stem string, opts scraper.ListOpts, out chan<- scraper.SceneResult) (cards []listCard, stoppedEarly bool) {
	seen := make(map[string]bool)
	for page := 1; page <= maxPages; page++ {
		if ctx.Err() != nil {
			return cards, false
		}
		if page > 1 && !sleep(ctx, opts.Delay) {
			return cards, false
		}

		u := fmt.Sprintf("%s%s/%s_%d_d.html", base, tourPrefix, stem, page)
		scraper.Debugf(1, "%s: fetching listing page %d", siteID, page)
		body, err := s.fetchPage(ctx, u)
		if err != nil {
			send(ctx, out, scraper.Error(fmt.Errorf("listing page %d: %w", page, err)))
			return cards, false
		}

		batch := parseCards(string(body))
		if len(batch) == 0 {
			return cards, false
		}

		added := 0
		for _, c := range batch {
			if seen[c.id] {
				continue
			}
			seen[c.id] = true
			added++
			// The listing is the CMS's own newest-first sort, so a stored id
			// means the remainder is already held.
			if opts.KnownIDs[c.id] {
				scraper.Debugf(1, "%s: hit known ID %s, stopping early", siteID, c.id)
				return cards, true
			}
			cards = append(cards, c)
		}
		if added == 0 {
			return cards, false
		}
	}
	return cards, false
}

func (s *Scraper) fetchDetails(ctx context.Context, studioURL, base string, cards []listCard, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	work := make(chan listCard)
	var wg sync.WaitGroup
	// LIFO: close(work) ends the range loops, then wg.Wait blocks until the
	// workers are gone, so a ctx.Done bail cannot leak them.
	defer wg.Wait()
	defer close(work)

	for i := 0; i < opts.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range work {
				if !sleep(ctx, opts.Delay) {
					return
				}
				scene, err := s.buildScene(ctx, studioURL, base, c)
				if err != nil {
					if !send(ctx, out, scraper.Error(err)) {
						return
					}
					continue
				}
				if !send(ctx, out, scraper.Scene(scene)) {
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

func (s *Scraper) buildScene(ctx context.Context, studioURL, base string, c listCard) (models.Scene, error) {
	sceneURL := base + c.path
	scene := models.Scene{
		ID:        c.id,
		SiteID:    siteID,
		StudioURL: studioURL,
		Title:     c.title,
		URL:       sceneURL,
		Studio:    studioName,
		ScrapedAt: time.Now().UTC(),
	}
	if c.thumb != "" {
		scene.Thumbnail = base + c.thumb
	}

	body, err := s.fetchPage(ctx, sceneURL)
	if err != nil {
		return models.Scene{}, fmt.Errorf("scene %s: %w", c.id, err)
	}
	d := parseDetail(string(body))
	if scene.Title == "" {
		scene.Title = d.title
	}
	scene.Description = d.description
	scene.Performers = d.performers
	scene.Tags = d.tags
	if t, ok := parseDate(d.date); ok {
		scene.Date = t
	}

	// Most scenes carry two permanent tiers, a cheaper rental and a purchase.
	// Only the purchase is recorded. Putting the rental in `Discounted` would
	// misuse a sale field for a standing price — with `IsOnSale` false it never
	// reaches `Effective`, and with it true every scene would look permanently
	// on sale and `LowestPrice` would track rentals rather than what the scene
	// costs to own. Rent-only scenes have no purchase tier, so there the rental
	// is the price.
	switch {
	case c.buy > 0:
		scene.AddPrice(models.PriceSnapshot{Date: scene.ScrapedAt, Regular: c.buy})
	case c.rent > 0:
		scene.AddPrice(models.PriceSnapshot{Date: scene.ScrapedAt, Regular: c.rent})
	}

	return scene, nil
}

func (s *Scraper) fetchPage(ctx context.Context, u string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		Method:  http.MethodGet,
		URL:     u,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}

// ---- listing cards ----

type listCard struct {
	id    string
	path  string // site-relative /vod/scenes/…_vids.html
	title string
	thumb string
	rent  float64
	buy   float64
}

var (
	cardSplitRe  = regexp.MustCompile(`<div class="update_details"`)
	setIDRe      = regexp.MustCompile(`data-setid="(\d+)"`)
	scenePathRe  = regexp.MustCompile(`href="(?:https?://[^"]*?)?(/vod/scenes/[^"?#]+_vids\.html)"`)
	cardTitleRe  = regexp.MustCompile(`data-title="([^"]*)"`)
	cardThumbRe  = regexp.MustCompile(`src0_4x="([^"]+)"`)
	cardThumb1Re = regexp.MustCompile(`src0_1x="([^"]+)"`)
	packagesRe   = regexp.MustCompile(`(?s)<div id="packageinfo_\d+"[^>]*>\s*(\{.*?\})\s*</div>`)
)

type vodPackage struct {
	Price string `json:"Price"`
}

type vodPackages struct {
	Rent []vodPackage `json:"rent"`
	Buy  []vodPackage `json:"buy"`
}

func parseCards(body string) []listCard {
	chunks := cardSplitRe.Split(body, -1)
	var out []listCard
	for i, chunk := range chunks {
		if i == 0 {
			continue // preamble before the first card
		}
		id := firstSubmatch(setIDRe, chunk)
		path := firstSubmatch(scenePathRe, chunk)
		if id == "" || path == "" {
			continue
		}
		c := listCard{
			id:    id,
			path:  path,
			title: cleanText(firstSubmatch(cardTitleRe, chunk)),
			thumb: firstNonEmpty(firstSubmatch(cardThumbRe, chunk), firstSubmatch(cardThumb1Re, chunk)),
		}
		c.rent, c.buy = parsePrices(chunk)
		out = append(out, c)
	}
	return out
}

// parsePrices reads the per-scene package JSON the cart widget carries. The
// visible "Buy $4.99 - $5.99" label is a range across both purchase types, so
// scraping the label would record the rental as the purchase price.
func parsePrices(chunk string) (rent, buy float64) {
	m := packagesRe.FindStringSubmatch(chunk)
	if m == nil {
		return 0, 0
	}
	var p vodPackages
	if err := json.Unmarshal([]byte(html.UnescapeString(m[1])), &p); err != nil {
		return 0, 0
	}
	return lowest(p.Rent), lowest(p.Buy)
}

func lowest(ps []vodPackage) float64 {
	best := 0.0
	for _, p := range ps {
		v, err := strconv.ParseFloat(p.Price, 64)
		if err != nil || v <= 0 {
			continue
		}
		if best == 0 || v < best {
			best = v
		}
	}
	return best
}

// ---- detail page ----

type detail struct {
	title       string
	date        string
	description string
	performers  []string
	tags        []string
}

var (
	pageTitleRe  = regexp.MustCompile(`(?s)<title>(.*?)</title>`)
	detailDateRe = regexp.MustCompile(`(?s)class="cell update_date">\s*(\d{2}/\d{2}/\d{4})`)
	descRe       = regexp.MustCompile(`(?s)<span class="update_description">(.*?)</span>`)
	modelsRe     = regexp.MustCompile(`(?s)<span class="update_models">(.*?)</span>`)
	tagsRe       = regexp.MustCompile(`(?s)<span class="update_tags">(.*?)</span>`)
	anchorTextRe = regexp.MustCompile(`(?s)<a[^>]*>(.*?)</a>`)
)

func parseDetail(body string) detail {
	d := detail{
		title:       cleanText(firstSubmatch(pageTitleRe, body)),
		date:        firstSubmatch(detailDateRe, body),
		description: cleanText(firstSubmatch(descRe, body)),
	}
	if span := firstSubmatch(modelsRe, body); span != "" {
		d.performers = anchorTexts(span)
	}
	if span := firstSubmatch(tagsRe, body); span != "" {
		d.tags = anchorTexts(span)
	}
	return d
}

// ---- helpers ----

var tagStripRe = regexp.MustCompile(`<[^>]*>`)

func cleanText(s string) string {
	s = tagStripRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, " ", " ")
	return strings.Join(strings.Fields(s), " ")
}

func anchorTexts(span string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, m := range anchorTextRe.FindAllStringSubmatch(span, -1) {
		t := cleanText(m[1])
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

func firstSubmatch(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func parseDate(s string) (time.Time, bool) {
	t, err := time.Parse("01/02/2006", s)
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
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
