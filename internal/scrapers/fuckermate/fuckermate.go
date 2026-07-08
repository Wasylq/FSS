// Package fuckermate scrapes fuckermate.com, a standalone Laravel/PHP gay studio
// site. The public video listing (/video, paginated via ?page=N) exposes each
// scene as an HTML card; the per-scene detail page (/video/{slug}) carries the
// absolute release date, tags, and cast — all public, none paywalled (only the
// video stream itself and the duration are members-only).
package fuckermate

import (
	"context"
	"fmt"
	"html"
	"net/http"
	neturl "net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const (
	siteID = "fuckermate"
	studio = "Fuckermate"
	base   = "https://www.fuckermate.com"
)

type Scraper struct {
	client *http.Client
	base   string // listing/detail host root; overridable in tests
}

func New() *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second), base: base}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{"fuckermate.com", "fuckermate.com/video", "fuckermate.com/video/{slug}"}
}

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

var matchRe = regexp.MustCompile(`(?i)^https?://(?:www\.)?fuckermate\.com`)

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, opts, out)
	return out, nil
}

func (s *Scraper) run(ctx context.Context, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		cards, hasNext, err := s.fetchListing(ctx, page)
		if err != nil {
			return scraper.PageResult{}, err
		}
		if len(cards) == 0 {
			return scraper.PageResult{Done: true}, nil
		}

		scenes := make([]models.Scene, 0, len(cards))
		for _, c := range cards {
			if ctx.Err() != nil {
				return scraper.PageResult{}, ctx.Err()
			}
			sc := models.Scene{
				ID:        c.slug,
				SiteID:    siteID,
				StudioURL: base,
				Title:     c.title,
				URL:       c.url,
				Thumbnail: c.thumbnail,
				Studio:    studio,
				ScrapedAt: now,
			}
			// Enrich with date, tags, and cast from the detail page. Fetch via
			// s.base (the listing hrefs are absolute to the live domain; routing
			// through s.base keeps the offline test self-contained).
			if d, derr := s.fetchDetail(ctx, s.base+c.path); derr == nil {
				sc.Date = d.date
				sc.Tags = d.tags
				sc.Performers = d.performers
				if d.title != "" {
					sc.Title = d.title
				}
			} else {
				scraper.Debugf(1, "%s: detail fetch failed for %s: %v", siteID, c.url, derr)
			}
			// No price snapshot: Fuckermate is a members-only paid site with no
			// public per-scene price, and scenes are not free — emitting a
			// zero-value snapshot would misrepresent the data.
			scenes = append(scenes, sc)
		}

		// rel="next" is the only reliable end-of-listing signal — the per-page
		// card count is not fixed (live pages return ~18, not a round 24).
		return scraper.PageResult{Scenes: scenes, Done: !hasNext}, nil
	})
}

// ---- listing ----

type card struct {
	slug      string
	title     string
	url       string // canonical absolute URL as presented by the site
	path      string // path component, used to fetch the detail page via s.base
	thumbnail string
}

var (
	cardRe = regexp.MustCompile(`(?s)<div class="post-thumbnail">\s*<a href="[^"]+">\s*<img src="([^"]+)"[^>]*>\s*</a>\s*</div>\s*<div class="post-header[^"]*">\s*<h1 class="post-title">\s*<a href="([^"]+)">([^<]+)</a>`)
	nextRe = regexp.MustCompile(`rel="next"`)
)

func (s *Scraper) fetchListing(ctx context.Context, page int) ([]card, bool, error) {
	u := fmt.Sprintf("%s/video?page=%d", s.base, page)
	scraper.Debugf(1, "%s: fetching listing page %d", siteID, page)
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     u,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return nil, false, fmt.Errorf("reading listing page %d: %w", page, err)
	}

	matches := cardRe.FindAllSubmatch(body, -1)
	cards := make([]card, 0, len(matches))
	for _, m := range matches {
		rawURL := string(m[2])
		cards = append(cards, card{
			slug:      slugFromURL(rawURL),
			title:     cleanText(string(m[3])),
			url:       rawURL,
			path:      pathFromURL(rawURL),
			thumbnail: string(m[1]),
		})
	}
	hasNext := nextRe.Match(body)
	scraper.Debugf(1, "%s: page %d has %d cards (next=%v)", siteID, page, len(cards), hasNext)
	return cards, hasNext, nil
}

// ---- detail ----

type detail struct {
	title      string
	date       time.Time
	tags       []string
	performers []string
}

var (
	detailTitleRe = regexp.MustCompile(`(?s)<h2 class="post-title">([^<]+)</h2>`)
	metaRe        = regexp.MustCompile(`(?s)<div class="post-meta">\s*(\d{4}-\d{2}-\d{2})\s*\|(.*?)</div>`)
	tagRe         = regexp.MustCompile(`<a href="[^"]*/video/tag/[^"]*">([^<]+)</a>`)
	// Performer links in the Cast section: /actor/{slug} with no role/ethnicity/
	// bodytype sub-path (those contain a slash, excluded by [a-z0-9-]+).
	performerRe = regexp.MustCompile(`href="[^"]*/actor/[a-z0-9-]+">([^<]+)</a>`)
)

func (s *Scraper) fetchDetail(ctx context.Context, sceneURL string) (detail, error) {
	scraper.Debugf(1, "%s: fetching detail %s", siteID, sceneURL)
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     sceneURL,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return detail{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return detail{}, fmt.Errorf("reading detail: %w", err)
	}
	return parseDetail(body), nil
}

func parseDetail(body []byte) detail {
	var d detail
	if m := detailTitleRe.FindSubmatch(body); m != nil {
		d.title = cleanText(string(m[1]))
	}
	if m := metaRe.FindSubmatch(body); m != nil {
		if t, err := time.Parse("2006-01-02", string(m[1])); err == nil {
			d.date = t
		}
		for _, tm := range tagRe.FindAllSubmatch(m[2], -1) {
			if v := cleanText(string(tm[1])); v != "" {
				d.tags = append(d.tags, v)
			}
		}
	}
	seen := make(map[string]bool)
	for _, pm := range performerRe.FindAllSubmatch(body, -1) {
		v := cleanText(string(pm[1]))
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		d.performers = append(d.performers, v)
	}
	return d
}

// ---- helpers ----

// pathFromURL strips scheme://host from an absolute URL, returning the leading
// "/..." path (with any query). Returns the input unchanged if it is already a
// path or cannot be parsed.
func pathFromURL(u string) string {
	if pu, err := neturl.Parse(u); err == nil && pu.Path != "" {
		p := pu.Path
		if pu.RawQuery != "" {
			p += "?" + pu.RawQuery
		}
		return p
	}
	return u
}

func slugFromURL(u string) string {
	u = strings.TrimRight(u, "/")
	if i := strings.LastIndex(u, "/"); i >= 0 {
		return u[i+1:]
	}
	return u
}

func cleanText(s string) string {
	return strings.TrimSpace(html.UnescapeString(s))
}
