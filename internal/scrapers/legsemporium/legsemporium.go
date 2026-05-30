package legsemporium

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/parseutil"
	"github.com/Wasylq/FSS/scraper"
)

const defaultBaseURL = "https://legsemporium.com"

type Scraper struct {
	base string
}

func New() *Scraper { return &Scraper{base: defaultBaseURL} }

// newWithBase is the test entrypoint: tests inject an httptest server URL
// instead of mutating package state. Production code uses [New].
func newWithBase(base string) *Scraper { return &Scraper{base: base} }

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return "legsemporium" }

func (s *Scraper) Patterns() []string {
	return []string{
		"legsemporium.com",
		"legsemporium.com/product-category/{category}",
	}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?legsemporium\.com(?:/|$)`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

type session struct {
	client *http.Client
	csrf   string
	base   string
}

type productEntry struct {
	id        string
	title     string
	url       string
	price     float64
	salePrice float64
	thumbnail string
	views     int
	likes     int
}

var (
	csrfRe       = regexp.MustCompile(`this\.csrf\s*=\s*"([^"]+)"`)
	dataIDRe     = regexp.MustCompile(`data-id="(\d+)"`)
	dataTitleRe  = regexp.MustCompile(`data-title="([^"]*)"`)
	dataPriceRe  = regexp.MustCompile(`data-price="([^"]*)"`)
	hrefRe       = regexp.MustCompile(`<a\s+href="(https?://[^"]*legsemporium\.com/product/[^"]*)"`)
	imgSrcRe     = regexp.MustCompile(`<img[^>]+class="a-img"[^>]+src="([^"]*)"`)
	viewsRe      = regexp.MustCompile(`icon-eye[^<]*</i>\s*<span>([^<]+)</span>`)
	likesRe      = regexp.MustCompile(`icon-clap[^<]*</i>\s*<span>([^<]+)</span>`)
	subcatURLRe  = regexp.MustCompile(`href="(https?://[^"]*legsemporium\.com/product-category/[^"]*)"`)
	modelCardRe  = regexp.MustCompile(`o-cat-item-models`)
	salePriceRe  = regexp.MustCompile(`<u>\$[\d.]+</u>\s*<span[^>]*class="[^"]*u-cl-red[^"]*"[^>]*>\$([^<]+)</span>`)
	durationRe   = regexp.MustCompile(`Duration\s+([\d:]+)`)
	tagRe        = regexp.MustCompile(`<a\s+href="https?://[^"]*legsemporium\.com/product-tag/[^"]*"\s+class="a-tag[^"]*">([^<]+)</a>`)
	breadcrumbRe = regexp.MustCompile(`<a[^>]+class="o-breadcrumbs-link"[^>]*>([^<]+)</a>`)
	posterRe     = regexp.MustCompile(`poster="([^"]+)"`)
)

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	scraper.Debugf(1, "bootstrap: fetching CSRF token from %s", s.base)
	sess, err := bootstrap(ctx, s.base)
	if err != nil {
		select {
		case out <- scraper.Error(fmt.Errorf("bootstrap: %w", err)):
		case <-ctx.Done():
		}

		return
	}
	scraper.Debugf(1, "bootstrap: CSRF token acquired (%d chars)", len(sess.csrf))

	slug := extractSlug(studioURL)
	scraper.Debugf(1, "slug: %q", slug)

	leaves, err := discoverLeaves(ctx, sess, slug, studioURL)
	if err != nil {
		select {
		case out <- scraper.Error(fmt.Errorf("discovering categories: %w", err)):
		case <-ctx.Done():
		}

		return
	}
	scraper.Debugf(1, "discovered %d leaf categories: %v", len(leaves), leaves)

	var entries []productEntry
	for _, leaf := range leaves {
		if ctx.Err() != nil {
			return
		}
		for _, tab := range []string{"video", "photo"} {
			products, err := paginateLeaf(ctx, sess, leaf, tab, opts)
			if err != nil {
				select {
				case out <- scraper.Error(fmt.Errorf("paginating %s/%s: %w", leaf, tab, err)):
				case <-ctx.Done():
					return
				}
				continue
			}
			scraper.Debugf(1, "  %s/%s: %d products", leaf, tab, len(products))
			entries = append(entries, products...)
		}
	}

	entries = dedup(entries)
	scraper.Debugf(1, "%d entries after dedup", len(entries))

	select {
	case out <- scraper.Progress(len(entries)):
	case <-ctx.Done():
		return
	}

	workers := opts.Workers
	if workers <= 0 {
		workers = 4
	}

	type detailResult struct {
		idx   int
		scene models.Scene
		err   error
	}

	work := make(chan int, len(entries))
	for i := range entries {
		work <- i
	}
	close(work)

	results := make(chan detailResult, len(entries))
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				if ctx.Err() != nil {
					return
				}
				if opts.Delay > 0 {
					select {
					case <-time.After(opts.Delay):
					case <-ctx.Done():
						return
					}
				}
				scene, err := fetchDetail(ctx, sess, entries[i], studioURL)
				results <- detailResult{idx: i, scene: scene, err: err}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		if ctx.Err() != nil {
			return
		}
		if r.err != nil {
			select {
			case out <- scraper.Error(r.err):
			case <-ctx.Done():
				return
			}
			continue
		}
		select {
		case out <- scraper.Scene(r.scene):
		case <-ctx.Done():
			return
		}
	}
}

func bootstrap(ctx context.Context, base string) (*session, error) {
	jar, _ := cookiejar.New(nil)
	client := httpx.NewClient(30 * time.Second)
	client.Jar = jar

	resp, err := httpx.Do(ctx, client, httpx.Request{
		URL:     base,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, fmt.Errorf("fetching homepage: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading homepage: %w", err)
	}

	m := csrfRe.FindSubmatch(body)
	if m == nil {
		return nil, fmt.Errorf("CSRF token not found")
	}

	return &session{client: client, csrf: string(m[1]), base: base}, nil
}

func extractSlug(u string) string {
	u = strings.TrimRight(u, "/")
	if i := strings.Index(u, "/product-category/"); i >= 0 {
		return strings.TrimRight(u[i+len("/product-category/"):], "/")
	}
	return ""
}

func discoverLeaves(ctx context.Context, sess *session, slug, studioURL string) ([]string, error) {
	if slug == "" {
		scraper.Debugf(1, "discover: no slug, scanning root category page")
		return discoverFromPage(ctx, sess, sess.base+"/product-category", 0)
	}

	scraper.Debugf(1, "discover: fetching %s/product-category/%s", sess.base, slug)
	body, err := fetchPage(ctx, sess, sess.base+"/product-category/"+slug)
	if err != nil {
		return nil, err
	}

	if modelCardRe.Match(body) {
		scraper.Debugf(1, "discover: %s has subcategories (model cards found)", slug)
		var leaves []string
		for _, m := range subcatURLRe.FindAllSubmatch(body, -1) {
			href := string(m[1])
			href = strings.TrimRight(href, "/")
			if href == studioURL || href == sess.base+"/product-category" {
				continue
			}
			subSlug := extractSlug(href)
			if subSlug == "" || subSlug == slug {
				continue
			}
			sub, err := discoverLeaves(ctx, sess, subSlug, studioURL)
			if err != nil {
				return nil, err
			}
			leaves = append(leaves, sub...)
			if ctx.Err() != nil {
				return leaves, ctx.Err()
			}
		}
		if len(leaves) > 0 {
			return leaves, nil
		}
	}

	return []string{slug}, nil
}

func discoverFromPage(ctx context.Context, sess *session, pageURL string, depth int) ([]string, error) {
	if depth > 5 {
		return nil, nil
	}
	body, err := fetchPage(ctx, sess, pageURL)
	if err != nil {
		return nil, err
	}

	if !modelCardRe.Match(body) {
		slug := extractSlug(pageURL)
		if slug != "" {
			return []string{slug}, nil
		}
		return nil, nil
	}

	var leaves []string
	seen := map[string]bool{}
	for _, m := range subcatURLRe.FindAllSubmatch(body, -1) {
		href := strings.TrimRight(string(m[1]), "/")
		if seen[href] || href == pageURL || href == sess.base+"/product-category" {
			continue
		}
		seen[href] = true
		sub, err := discoverFromPage(ctx, sess, href, depth+1)
		if err != nil {
			return nil, err
		}
		leaves = append(leaves, sub...)
		if ctx.Err() != nil {
			return leaves, ctx.Err()
		}
	}
	return leaves, nil
}

func fetchPage(ctx context.Context, sess *session, u string) ([]byte, error) {
	resp, err := httpx.Do(ctx, sess.client, httpx.Request{
		URL:     u,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}

type ajaxResponse struct {
	HTMLMod []struct {
		Value string `json:"value"`
	} `json:"htmlMod"`
	IsLast   bool `json:"isLast"`
	NextPage int  `json:"nextPage"`
}

// fetchAjaxPage POSTs one paginated AJAX request and decodes the response.
// Kept as its own function so `defer resp.Body.Close()` fires per page — the
// previous inline loop leaked one connection per page on long catalogs.
func fetchAjaxPage(ctx context.Context, sess *session, body string, headers map[string]string) (ajaxResponse, error) {
	var ar ajaxResponse
	resp, err := httpx.Do(ctx, sess.client, httpx.Request{
		URL:     sess.base + "/product-category",
		Body:    []byte(body),
		Headers: headers,
	})
	if err != nil {
		return ar, err
	}
	defer func() { _ = resp.Body.Close() }()
	if err := httpx.DecodeJSON(resp.Body, &ar); err != nil {
		return ar, fmt.Errorf("decode: %w", err)
	}
	return ar, nil
}

func paginateLeaf(ctx context.Context, sess *session, slug, tab string, opts scraper.ListOpts) ([]productEntry, error) {
	var all []productEntry
	page := 1

	for {
		if ctx.Err() != nil {
			return all, ctx.Err()
		}

		scraper.Debugf(1, "paginate: %s/%s page %d", slug, tab, page)

		body := fmt.Sprintf("page=%d&start=%d&per_page=48&slug=/product-category/%s/%s&sort=1&more=0",
			page, page, slug, tab)

		h := map[string]string{
			"User-Agent":       httpx.UserAgentFirefox,
			"X-Requested-With": "XMLHttpRequest",
			"X-CSRF-TOKEN":     sess.csrf,
			"Content-Type":     "application/x-www-form-urlencoded",
			"Referer":          sess.base + "/product-category/" + slug,
		}

		ar, err := fetchAjaxPage(ctx, sess, body, h)
		if err != nil {
			return all, fmt.Errorf("page %d: %w", page, err)
		}

		if len(ar.HTMLMod) == 0 {
			break
		}

		html := ar.HTMLMod[0].Value
		entries := parseProductCards(html, sess.base)
		scraper.Debugf(1, "paginate: page %d → %d cards, isLast=%v, nextPage=%d", page, len(entries), ar.IsLast, ar.NextPage)
		if len(entries) == 0 {
			break
		}

		stopped := false
		for i := range entries {
			if entries[i].id != "" && opts.KnownIDs[entries[i].id] {
				all = append(all, entries[:i]...)
				stopped = true
				break
			}
		}
		if stopped {
			return all, nil
		}

		all = append(all, entries...)

		if ar.IsLast {
			break
		}
		page = ar.NextPage
		if page == 0 {
			break
		}

		if opts.Delay > 0 {
			select {
			case <-time.After(opts.Delay):
			case <-ctx.Done():
				return all, ctx.Err()
			}
		}
	}

	return all, nil
}

func parseProductCards(html, base string) []productEntry {
	ids := dataIDRe.FindAllStringSubmatch(html, -1)
	titles := dataTitleRe.FindAllStringSubmatch(html, -1)
	prices := dataPriceRe.FindAllStringSubmatch(html, -1)
	hrefs := hrefRe.FindAllStringSubmatch(html, -1)

	n := len(ids)
	if n == 0 {
		return nil
	}

	entries := make([]productEntry, n)
	for i := range n {
		entries[i].id = ids[i][1]
		if i < len(titles) {
			entries[i].title = decodeHTMLEntities(titles[i][1])
		}
		if i < len(prices) {
			entries[i].price, _ = strconv.ParseFloat(prices[i][1], 64)
		}
		if i < len(hrefs) {
			entries[i].url = hrefs[i][1]
		}
	}

	imgs := imgSrcRe.FindAllStringSubmatch(html, -1)
	viewsList := viewsRe.FindAllStringSubmatch(html, -1)
	likesList := likesRe.FindAllStringSubmatch(html, -1)

	for i := range entries {
		if i < len(imgs) {
			src := imgs[i][1]
			if !strings.HasPrefix(src, "http") {
				src = base + src
			}
			entries[i].thumbnail = src
		}
		if i < len(viewsList) {
			entries[i].views = parseCount(viewsList[i][1])
		}
		if i < len(likesList) {
			entries[i].likes = parseCount(likesList[i][1])
		}
	}

	salePrices := salePriceRe.FindAllStringSubmatch(html, -1)
	for i := range entries {
		if i < len(salePrices) {
			entries[i].salePrice, _ = strconv.ParseFloat(strings.TrimSpace(salePrices[i][1]), 64)
		}
	}

	return entries
}

func fetchDetail(ctx context.Context, sess *session, e productEntry, studioURL string) (models.Scene, error) {
	scene := models.Scene{
		ID:        e.id,
		SiteID:    "legsemporium",
		StudioURL: studioURL,
		Title:     e.title,
		URL:       e.url,
		Thumbnail: e.thumbnail,
		Views:     e.views,
		Likes:     e.likes,
		ScrapedAt: time.Now().UTC(),
	}

	if e.price > 0 {
		regular := e.price
		snap := models.PriceSnapshot{
			Date:    scene.ScrapedAt,
			Regular: regular,
		}
		if e.salePrice > 0 && e.salePrice < regular {
			snap.IsOnSale = true
			snap.Discounted = e.salePrice
			pct := int(((regular - e.salePrice) / regular) * 100)
			snap.DiscountPercent = pct
		}
		scene.AddPrice(snap)
	}

	if e.url == "" {
		return scene, nil
	}

	body, err := fetchPage(ctx, sess, e.url)
	if err != nil {
		return scene, nil
	}

	if m := durationRe.FindSubmatch(body); m != nil {
		scene.Duration = parseutil.ParseDurationColon(string(m[1]))
	}

	for _, m := range tagRe.FindAllSubmatch(body, -1) {
		scene.Tags = append(scene.Tags, strings.TrimSpace(string(m[1])))
	}

	crumbs := breadcrumbRe.FindAllSubmatch(body, -1)
	if len(crumbs) >= 3 {
		scene.Performers = []string{strings.TrimSpace(string(crumbs[2][1]))}
	}
	if len(crumbs) >= 2 {
		scene.Categories = []string{strings.TrimSpace(string(crumbs[1][1]))}
	}

	if scene.Thumbnail == "" {
		if m := posterRe.FindSubmatch(body); m != nil {
			src := string(m[1])
			if !strings.HasPrefix(src, "http") {
				src = sess.base + src
			}
			scene.Thumbnail = src
		}
	}

	return scene, nil
}

func dedup(entries []productEntry) []productEntry {
	seen := make(map[string]bool, len(entries))
	out := make([]productEntry, 0, len(entries))
	for _, e := range entries {
		if e.id != "" && seen[e.id] {
			continue
		}
		seen[e.id] = true
		out = append(out, e)
	}
	return out
}

func parseCount(s string) int {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	multiplier := 1
	if strings.HasSuffix(s, "k") {
		s = s[:len(s)-1]
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		return int(f * 1000)
	}
	n, _ := strconv.Atoi(s)
	return n * multiplier
}

func decodeHTMLEntities(s string) string {
	r := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&#039;", "'",
		"&quot;", `"`,
		"&#39;", "'",
	)
	return r.Replace(s)
}
