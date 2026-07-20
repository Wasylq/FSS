// Package nextcontents scrapes a family of Next.js tour sites (FreakMob Media,
// Deepthroat Sirens, Swallowed) that share one CMS: an mjedge.net thumbnail CDN,
// NATS integration, and a `pageProps.contents` JSON payload served from
// /_next/data/{buildId}/{listPath}.json?page=N. The buildId rotates on redeploy
// and is scraped from the listing page's __NEXT_DATA__ before paginating.
package nextcontents

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
	"github.com/Wasylq/FSS/parseutil"
	"github.com/Wasylq/FSS/scraper"
)

type siteConfig struct {
	SiteID    string
	Studio    string
	Base      string // tour origin, e.g. https://www.freakmobmedia.com
	ListPath  string // JSON/listing route stem: "videos" or "scenes"
	BrandSlug string // model slug for the studio's own brand, dropped from performers
}

var sites = []siteConfig{
	{"freakmob", "FreakMob Media", "https://www.freakmobmedia.com", "videos", "freakmob"},
	{"deepthroatsirens", "Deepthroat Sirens", "https://tour.deepthroatsirens.com", "scenes", ""},
	{"swallowed", "Swallowed", "https://tour.swallowed.com", "scenes", ""},

	// Sticky Dollars network (stickydollars.com is the hub linking these).
	//
	// Dirty Auditions is served from its APEX domain, not a tour subdomain:
	// tour.dirtyauditions.com is a misconfigured host serving a completely
	// different site (AD4X, site_domain ad4x.com, 1309 scenes on a /videos
	// route). Pointing Base there would silently scrape the wrong catalogue.
	{"trueanal", "True Anal", "https://tour.trueanal.com", "scenes", ""},
	{"nympho", "Nympho", "https://tour.nympho.com", "scenes", ""},
	{"dirtyauditions", "Dirty Auditions", "https://dirtyauditions.com", "scenes", ""},
	{"allanal", "All Anal", "https://tour.allanal.com", "scenes", ""},
	{"analonly", "Anal Only", "https://tour.analonly.com", "scenes", ""},

	// Top Web Models network. Six brands have their own tour host serving a
	// /scenes route; the rest exist only as /sites/{domain} on the hub, which
	// returns the same pageProps.contents payload filtered to that brand.
	// Deepthroat Sirens is part of this network too but is already configured
	// above under its own entry.
	{"biggulpgirls", "Big Gulp Girls", "https://tour.biggulpgirls.com", "scenes", ""},
	{"shesbrandnew", "She's Brand New", "https://tour.shesbrandnew.com", "scenes", ""},
	{"facialsforever", "Facials Forever", "https://tour.facialsforever.com", "scenes", ""},
	{"cougarseason", "Cougar Season", "https://tour.cougarseason.com", "scenes", ""},
	{"poundedpetite", "Pounded Petite", "https://tour.poundedpetite.com", "scenes", ""},
	{"2girls1camera", "2 Girls 1 Camera", "https://tour.2girls1camera.com", "scenes", ""},

	// Hub-only brands — no live domain of their own, so Base is the hub and
	// ListPath selects the per-site route. Scene URLs still resolve under the
	// hub's /scenes/{slug}.
	{"topwebmodels", "Top Web Models", "https://tour.topwebmodels.com", "sites/topwebmodels.com", ""},
	{"twmclassics", "TWM Classics", "https://tour.topwebmodels.com", "sites/twmclassics.com", ""},
	{"twminterviews", "TWM Interviews", "https://tour.topwebmodels.com", "sites/topwebmodels-interviews.com", ""},
	{"twmpornvault", "TWM Porn Vault", "https://tour.topwebmodels.com", "sites/twm-porn-vault.com", ""},

	// AltErotic serves the same CMS from its apex domain on a /videos route.
	// Its eight sub-brands (Alt Fetish, Director POV, Ink Amateurs, Inked Up
	// Sex, My Tattoo Girls, Piercing Play, Sexy Inked and the Original Series)
	// are channels on this host, not separate domains.
	{"alterotic", "AltErotic", "https://alterotic.com", "videos", ""},

	// Blake Mason, the surviving Twisted XXX Media brand (that network's
	// domain no longer resolves; the site is run by IndieBucks now). Its
	// payload leaves seconds_duration null and uses the older `thumbnail`
	// field, both of which toScene falls back to.
	{"blakemason", "Blake Mason", "https://blakemason.com", "videos", ""},
}

type Scraper struct {
	cfg     siteConfig
	client  *http.Client
	matchRe *regexp.Regexp
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func newScraper(cfg siteConfig) *Scraper {
	host := strings.TrimPrefix(cfg.Base, "https://")
	host = strings.TrimPrefix(host, "http://")
	pattern := `^https?://` + strings.ReplaceAll(host, ".", `\.`)

	// Brands with no domain of their own live at {hub}/sites/{domain}, so
	// several configs share one Base. Matching on the host alone would make
	// them indistinguishable — and would let any of them answer for the bare
	// hub URL, which is an aggregate of other brands. Scope those to their
	// own path instead; the hub root deliberately matches no scraper.
	if strings.HasPrefix(cfg.ListPath, "sites/") {
		pattern += `/` + strings.ReplaceAll(cfg.ListPath, ".", `\.`) + `(?:/|$)`
	}

	return &Scraper{
		cfg:     cfg,
		client:  httpx.NewClient(30 * time.Second),
		matchRe: regexp.MustCompile(pattern),
	}
}

func init() {
	for _, cfg := range sites {
		scraper.Register(newScraper(cfg))
	}
}

func (s *Scraper) ID() string { return s.cfg.SiteID }

func (s *Scraper) Patterns() []string {
	host := strings.TrimPrefix(strings.TrimPrefix(s.cfg.Base, "https://"), "http://")
	return []string{host, host + "/" + s.cfg.ListPath}
}

func (s *Scraper) MatchesURL(u string) bool { return s.matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, opts, out)
	return out, nil
}

var buildIDRe = regexp.MustCompile(`"buildId":"([^"]+)"`)

func (s *Scraper) run(ctx context.Context, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	buildID, err := s.fetchBuildID(ctx)
	if err != nil {
		select {
		case out <- scraper.Error(err):
		case <-ctx.Done():
		}
		return
	}
	scraper.Debugf(1, "%s: buildId=%s", s.cfg.SiteID, buildID)

	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, s.cfg.SiteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		contents, err := s.fetchPage(ctx, buildID, page)
		if err != nil {
			return scraper.PageResult{}, err
		}
		scenes := make([]models.Scene, 0, len(contents.Data))
		for _, item := range contents.Data {
			scenes = append(scenes, s.toScene(item, now))
		}
		return scraper.PageResult{
			Scenes: scenes,
			Total:  contents.Total,
			Done:   contents.TotalPages > 0 && page >= contents.TotalPages,
		}, nil
	})
}

func (s *Scraper) fetchBuildID(ctx context.Context) (string, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     s.cfg.Base + "/" + s.cfg.ListPath,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return "", err
	}
	m := buildIDRe.FindSubmatch(body)
	if m == nil {
		return "", fmt.Errorf("%s: buildId not found in listing page", s.cfg.SiteID)
	}
	return string(m[1]), nil
}

func (s *Scraper) fetchPage(ctx context.Context, buildID string, page int) (contentsPage, error) {
	u := fmt.Sprintf("%s/_next/data/%s/%s.json?page=%d", s.cfg.Base, buildID, s.cfg.ListPath, page)
	scraper.Debugf(1, "%s: fetching page %d", s.cfg.SiteID, page)
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     u,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return contentsPage{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	var body nextData
	if err := httpx.DecodeJSON(resp.Body, &body); err != nil {
		return contentsPage{}, fmt.Errorf("decoding %s page %d: %w", s.cfg.SiteID, page, err)
	}
	return body.PageProps.Contents, nil
}

func (s *Scraper) toScene(item contentItem, now time.Time) models.Scene {
	sc := models.Scene{
		ID:          fmt.Sprintf("%d", item.ID),
		SiteID:      s.cfg.SiteID,
		StudioURL:   s.cfg.Base,
		Title:       html.UnescapeString(strings.TrimSpace(item.Title)),
		URL:         fmt.Sprintf("%s/%s/%s", s.cfg.Base, s.scenePath(), item.Slug),
		Description: html.UnescapeString(strings.TrimSpace(item.Description)),
		Studio:      s.cfg.Studio,
		Thumbnail:   thumbnail(item),
		Duration:    duration(item),
		Tags:        cleanTags(item.Tags),
		Performers:  s.performers(item),
		ScrapedAt:   now,
	}
	if t := parsePublishDate(item.PublishDate); !t.IsZero() {
		sc.Date = t
	}
	if item.ContentPrice > 0 {
		date := sc.Date
		if date.IsZero() {
			date = now
		}
		sc.AddPrice(models.PriceSnapshot{Date: date, Regular: float64(item.ContentPrice)})
	}
	return sc
}

// thumbnail prefers the newer `thumb` field, falling back to the older
// `thumbnail`, which is protocol-relative.
func thumbnail(item contentItem) string {
	if item.Thumb != "" {
		return item.Thumb
	}
	if strings.HasPrefix(item.Thumbnail, "//") {
		return "https:" + item.Thumbnail
	}
	return item.Thumbnail
}

// duration prefers seconds_duration, which some sites leave null. The display
// runtime is the fallback, in either of the two formats sites use for it.
func duration(item contentItem) int {
	if item.SecondsDuration > 0 {
		return item.SecondsDuration
	}
	v := strings.TrimSpace(item.VideosDuration)
	if v == "" {
		return 0
	}
	if strings.Contains(v, ":") {
		return parseutil.ParseDurationColon(v)
	}
	secs, err := strconv.ParseFloat(v, 64)
	if err != nil || secs < 0 {
		return 0
	}
	return int(secs)
}

// scenePath is the route detail pages live under. It matches ListPath on sites
// that serve their own catalogue — /videos/{slug} for a "videos" list, and
// likewise for "scenes". Hub-only brands use a "sites/{domain}" list path but
// their scenes still resolve under the hub's /scenes/{slug}.
func (s *Scraper) scenePath() string {
	if strings.Contains(s.cfg.ListPath, "/") {
		return "scenes"
	}
	return s.cfg.ListPath
}

// cleanTags trims and dedupes. The CMS prefixes many tags with a non-breaking
// space (e.g. "\u00a0arm Tattoo"), so the raw values cannot be used as-is.
// strings.TrimSpace handles U+00A0 — unicode.IsSpace covers it.
func cleanTags(tags []string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// performers returns the model names with the studio's own brand entry removed.
func (s *Scraper) performers(item contentItem) []string {
	var out []string
	for _, m := range item.ModelsSlugs {
		if s.cfg.BrandSlug != "" && strings.EqualFold(m.Slug, s.cfg.BrandSlug) {
			continue
		}
		name := strings.TrimSpace(m.Name)
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

// parsePublishDate parses the CMS date format "2026/06/13 12:00:00" to UTC.
func parsePublishDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse("2006/01/02 15:04:05", s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

type nextData struct {
	PageProps struct {
		Contents contentsPage `json:"contents"`
	} `json:"pageProps"`
}

type contentsPage struct {
	Total      int           `json:"total"`
	TotalPages int           `json:"total_pages"`
	Data       []contentItem `json:"data"`
}

type contentItem struct {
	ID              int64  `json:"id"`
	Title           string `json:"title"`
	Slug            string `json:"slug"`
	PublishDate     string `json:"publish_date"`
	SecondsDuration int    `json:"seconds_duration"`
	// VideosDuration is the display runtime. Sites disagree on its format —
	// "22:17" on some, float seconds ("1964.77") on others — and some leave
	// seconds_duration null, so it is only consulted as a fallback.
	VideosDuration string `json:"videos_duration"`
	Thumb          string `json:"thumb"`
	// Thumbnail is the older field name, and is protocol-relative.
	Thumbnail    string      `json:"thumbnail"`
	Description  string      `json:"description"`
	ContentPrice int         `json:"content_price"`
	Tags         []string    `json:"tags"`
	ModelsSlugs  []modelSlug `json:"models_slugs"`
}

type modelSlug struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}
