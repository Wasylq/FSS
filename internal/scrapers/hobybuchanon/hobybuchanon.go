// Package hobybuchanon scrapes hobybuchanon.com, a Next.js paysite tour that
// ships its listing data in a `<script id="__NEXT_DATA__">` JSON blob.
//
// The site presents four listings — `/updates`, `/suck-this-dick`,
// `/behind-the-scenes` and the `/hobyshotties` model index — but only the
// first is walked: `/updates` is a strict superset. Harvesting all three
// content listings in full returns 315 items and 315 distinct ids, with the
// 32 Suck This Dick and 28 BTS scenes all present in `/updates` as well. The
// sub-brand survives as `Series` (the payload's `site` field), so nothing is
// lost by walking one listing instead of three.
//
// Every scene resolves under `/updates/{slug}` regardless of which listing it
// also appears in, so the stored URL needs no per-brand path.
package hobybuchanon

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

	"github.com/Anastylosis/FSS/internal/httpx"
	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/scraper"
)

const (
	siteID     = "hobybuchanon"
	domain     = "hobybuchanon.com"
	studioName = "Hoby Buchanon"
	// hubSite is the payload's `site` value for scenes that belong to the main
	// brand rather than a sub-brand. Only the sub-brands become Series.
	hubSite = "hobybuchanon"
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

var (
	matchRe = regexp.MustCompile(`^https?://(?:www\.)?hobybuchanon\.com(?:/|$)`)
	modelRe = regexp.MustCompile(`/hobyshotties/([^/?#]+)`)
)

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		domain,
		domain + "/updates",
		domain + "/hobyshotties/{slug}",
	}
}

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// base resolves the origin to fetch from. The operator's own host wins so a
// non-www spelling stays addressable; baseOverride is the test server. Every
// request is built as base + path so pointing base at httptest redirects the
// whole crawl.
func (s *Scraper) base(studioURL string) string {
	if s.baseOverride != "" {
		return strings.TrimSuffix(s.baseOverride, "/")
	}
	if u, err := url.Parse(studioURL); err == nil && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}
	return "https://" + domain
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	base := s.base(studioURL)

	// The model index itself is /hobyshotties with no slug — that page carries
	// models, not scenes, so it falls through to the full listing.
	if m := modelRe.FindStringSubmatch(studioURL); m != nil && m[1] != "" {
		s.runModel(ctx, studioURL, base, m[1], out)
		return
	}

	scraper.Debugf(1, "%s: scraping full /updates listing", siteID)
	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := fmt.Sprintf("%s/updates?page=%d", base, page)
		pp, err := s.fetchPageProps(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}
		c := pp.Contents
		scenes := make([]models.Scene, 0, len(c.Data))
		for _, item := range c.Data {
			scenes = append(scenes, s.toScene(item, studioURL, base, now))
		}
		total := 0
		if page == 1 {
			total = int(c.Total)
		}
		return scraper.PageResult{
			Scenes: scenes,
			Total:  total,
			Done:   c.TotalPages > 0 && page >= int(c.TotalPages),
		}, nil
	})
}

// runModel reads a performer page. `model_contents` carries that model's whole
// filmography in one payload — it matches `published_content_ids` exactly on
// every page sampled — so there is nothing to paginate.
func (s *Scraper) runModel(ctx context.Context, studioURL, base, slug string, out chan<- scraper.SceneResult) {
	scraper.Debugf(1, "%s: scraping model %q", siteID, slug)

	pp, err := s.fetchPageProps(ctx, base+"/hobyshotties/"+slug)
	if err != nil {
		send(ctx, out, scraper.Error(fmt.Errorf("model %s: %w", slug, err)))
		return
	}
	if len(pp.ModelContents) == 0 {
		send(ctx, out, scraper.Error(scraper.ParseError(base+"/hobyshotties/"+slug,
			fmt.Errorf("model page carries no model_contents"))))
		return
	}

	now := time.Now().UTC()
	if !send(ctx, out, scraper.Progress(len(pp.ModelContents))) {
		return
	}
	for _, item := range pp.ModelContents {
		if !send(ctx, out, scraper.Scene(s.toScene(item, studioURL, base, now))) {
			return
		}
	}
}

// --- payload ---

var nextDataRe = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__"[^>]*>(.*?)</script>`)

type pageProps struct {
	Contents      contents      `json:"contents"`
	ModelContents []contentItem `json:"model_contents"`
}

type contents struct {
	Total      flexInt       `json:"total"`
	Page       flexInt       `json:"page"`
	PerPage    flexInt       `json:"per_page"`
	TotalPages flexInt       `json:"total_pages"`
	Data       []contentItem `json:"data"`
}

// flexInt accepts a JSON number or a quoted decimal string. The tour is
// inconsistent about which it emits: `/updates` returns `page` as a number,
// `/updates?page=2` returns the same field quoted, so a plain int fails the
// whole listing parse from page one onwards. Unparseable input zeroes rather
// than erroring — one odd pagination field must not cost the page its scenes.
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		*f = 0
		return nil //nolint:nilerr // deliberate: be lenient about one field
	}
	*f = flexInt(n)
	return nil
}

type contentItem struct {
	ID              flexInt     `json:"id"`
	Title           string      `json:"title"`
	Slug            string      `json:"slug"`
	Site            string      `json:"site"`
	PublishDate     string      `json:"publish_date"`
	SecondsDuration flexInt     `json:"seconds_duration"`
	Description     string      `json:"description"`
	Thumb           string      `json:"thumb"`
	TrailerURL      string      `json:"trailer_url"`
	Tags            []string    `json:"tags"`
	Models          []string    `json:"models"`
	ModelsSlugs     []modelSlug `json:"models_slugs"`
	Views           flexInt     `json:"views"`
}

type modelSlug struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func (s *Scraper) fetchPageProps(ctx context.Context, u string) (pageProps, error) {
	body, err := s.fetch(ctx, u)
	if err != nil {
		return pageProps{}, err
	}
	return parsePageProps(body, u)
}

func parsePageProps(body []byte, u string) (pageProps, error) {
	m := nextDataRe.FindSubmatch(body)
	if m == nil {
		return pageProps{}, scraper.ParseError(u, fmt.Errorf("no __NEXT_DATA__ block"))
	}
	var payload struct {
		Props struct {
			PageProps pageProps `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal(m[1], &payload); err != nil {
		return pageProps{}, scraper.ParseError(u, fmt.Errorf("decoding __NEXT_DATA__: %w", err))
	}
	return payload.Props.PageProps, nil
}

func (s *Scraper) fetch(ctx context.Context, u string) ([]byte, error) {
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

// publishDateLayout matches "2026/08/14 12:00:00" as emitted by the payload.
const publishDateLayout = "2006/01/02 15:04:05"

func (s *Scraper) toScene(item contentItem, studioURL, base string, now time.Time) models.Scene {
	scene := models.Scene{
		ID:          strconv.Itoa(int(item.ID)),
		SiteID:      siteID,
		StudioURL:   studioURL,
		Title:       cleanText(item.Title),
		URL:         base + "/updates/" + item.Slug,
		Studio:      studioName,
		Description: cleanText(item.Description),
		Thumbnail:   item.Thumb,
		Preview:     item.TrailerURL,
		Duration:    int(item.SecondsDuration),
		Tags:        cleanNames(item.Tags),
		Performers:  performers(item),
		ScrapedAt:   now,
	}
	// The payload's `site` is the sub-brand a scene was published under. Only
	// record it when it is not the hub, so the majority of scenes carry no
	// redundant Series equal to the studio.
	if site := strings.TrimSpace(item.Site); site != "" && !strings.EqualFold(site, hubSite) {
		scene.Series = site
	}
	if d, err := time.Parse(publishDateLayout, item.PublishDate); err == nil {
		scene.Date = d.UTC()
	}
	if item.Views > 0 {
		scene.Views = int(item.Views)
	}
	return scene
}

// performers prefers models_slugs, whose entries are the credits the site links
// to a model page; `models` is the same list flattened and is the fallback for
// a payload that omits the richer form.
func performers(item contentItem) []string {
	var out []string
	for _, ms := range item.ModelsSlugs {
		if n := cleanText(ms.Name); n != "" {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		out = cleanNames(item.Models)
	}
	return out
}

func cleanNames(in []string) []string {
	var out []string
	for _, v := range in {
		if n := cleanText(v); n != "" {
			out = append(out, n)
		}
	}
	return out
}

var tagStripRe = regexp.MustCompile(`<[^>]*>`)

func cleanText(s string) string {
	s = tagStripRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, " ", " ")
	return strings.Join(strings.Fields(s), " ")
}

func send(ctx context.Context, out chan<- scraper.SceneResult, r scraper.SceneResult) bool {
	select {
	case out <- r:
		return true
	case <-ctx.Done():
		return false
	}
}
