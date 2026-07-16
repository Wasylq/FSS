// Package natscmsutil scrapes sites running the "My Gay Cash" /
// TooMuchMedia NATS tour CMS — an Angular SPA mounted at `/natscms-app/`
// (`<base href="/natscms-app/">`, `nats-tours-root` body tag) backed by a
// `tour_api.php` JSON API. Each site's `/natscms-app/config.json` carries
// a `natsUrl` (the API host — a shared `nats.mygaycash.com` for the Mars
// Media network, or a self-hosted host like `api.cosplayground.com`) and a
// per-site `cms_area_id` UUID.
//
// This package holds the shared discovery/parse logic; concrete site
// tables (marsmedia, cosplayground, …) supply per-instance SiteConfig rows
// and register the scrapers.
//
// Discovery flow:
//
//  1. Fetch `{NatsAPIBase}/content/page?slug=/` with the
//     `X-NATS-cms-area-id: {uuid}` header (the API rejects the request as
//     "Invalid Area" without it). Walk the returned page's `blocks` array,
//     find the first `set_list` block, and capture its `cms_block_id`.
//  2. Fetch `…/content/servers` (same header) to get the
//     `cms_content_server_id → settings.url` map used to resolve thumbnail
//     CDN hostnames (e.g. `c76161b613.mjedge.net`).
//  3. Fetch `…/content/sets?cms_block_id={id}` (same header). The response
//     carries `total_count` plus a `sets` array containing every scene in
//     the block — the `max_asset_count` setting on the block is a
//     client-side render hint, not a server-side limit, so one request
//     returns the entire catalogue.
//
// Each set entry has:
//
//   - `cms_set_id` — stable scene ID
//   - `name` — title
//   - `description` — HTML-decoded scene synopsis (may be absent)
//   - `slug` — URL slug
//   - `added_nice` — publish date `YYYY-MM-DD`
//   - `member_views` — view count
//   - `preview_formatted.thumb.{ratio}[]` — per-ratio thumbnail variants,
//     each `{cms_content_server_id, fileuri, signature, ...}`. The final
//     URL is `{servers[id].settings.url}{fileuri}?{signature}` (trailing
//     slash on the server url is stripped).
//
// Detail pages are not fetched — every field is already on the listing.
// Scene URLs are synthesised as `{base}/tour/trailer/{slug}/`, the SPA's
// user-facing trailer route, so each scene has a stable anchor. Sets are
// returned newest-first, so date-sorted `KnownIDs` early-stop works.
package natscmsutil

import (
	"context"
	"encoding/json"
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

// maxSetsBytes caps the /content/sets read. The endpoint returns a site's
// entire catalogue in one response, which for a large archive runs well past
// httpx.MaxPageBytes — LadyboyGold's 5006 sets are ~11.5 MB — and a truncated
// read yields zero scenes rather than a partial list.
const maxSetsBytes = 64 * 1024 * 1024

// SiteConfig describes one NATS-CMS site.
type SiteConfig struct {
	ID       string
	SiteBase string // e.g. "https://www.bearfilms.com" — no trailing slash
	SiteName string
	// StudioName is written to Scene.Studio (the network/brand name, e.g.
	// "Mars Media" or "Cosplayground").
	StudioName string
	// NatsAPIBase is the site's `tour_api.php` base URL, e.g.
	// "https://nats.mygaycash.com/tour_api.php" (shared Mars Media backend)
	// or "https://api.cosplayground.com/tour_api.php" (self-hosted). Taken
	// from the site's `/natscms-app/config.json` `natsUrl` + `/tour_api.php`.
	NatsAPIBase string
	// CMSAreaID is the per-site UUID the NATS API uses as the
	// `X-NATS-cms-area-id` header. Hard-coded from each site's
	// `/natscms-app/config.json` to skip one HTTP round-trip per scrape.
	CMSAreaID string
	Patterns  []string
	MatchRe   *regexp.Regexp
	// SkipPathRe, when set, drops sets whose `path` matches it. Some sites
	// return photo sets and videos from the same block with no type field —
	// LadyboyGold's block mixes ~1800 photo sets into ~3200 scenes — and the
	// path segment is the only thing that tells them apart. Nil means keep
	// every set, which is the right default for sites that only publish video.
	SkipPathRe *regexp.Regexp
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

var _ scraper.StudioScraper = (*Scraper)(nil)

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

// ---- NATS API types ----

type pageResponse struct {
	Slug    string      `json:"slug"`
	Name    string      `json:"name"`
	Blocks  []pageBlock `json:"blocks"`
	Success bool        `json:"success"`
	Error   string      `json:"error,omitempty"`
}

type pageBlock struct {
	CMSBlockID string       `json:"cms_block_id"`
	Settings   blockSetting `json:"settings"`
}

type blockSetting struct {
	Type string `json:"type"`
}

type setsResponse struct {
	TotalCount stringOrInt `json:"total_count"`
	Sets       []setEntry  `json:"sets"`
	Success    bool        `json:"success"`
	Error      string      `json:"error,omitempty"`
}

type serversResponse struct {
	Servers []serverEntry `json:"servers"`
	Success bool          `json:"success"`
	Error   string        `json:"error,omitempty"`
}

type serverEntry struct {
	CMSContentServerID string         `json:"cms_content_server_id"`
	Name               string         `json:"name"`
	Settings           serverSettings `json:"settings"`
}

type serverSettings struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// stringOrInt accepts JSON values that may be either an int or a quoted
// string number (the NATS API emits `"total_count":"1473"` as a string).
type stringOrInt int

func (s *stringOrInt) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		*s = 0
		return nil
	}
	str := string(b)
	if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	}
	if str == "" {
		*s = 0
		return nil
	}
	n, err := strconv.Atoi(str)
	if err != nil {
		*s = 0
		return nil //nolint:nilerr // intentional leniency
	}
	*s = stringOrInt(n)
	return nil
}

type setEntry struct {
	CMSSetID    string      `json:"cms_set_id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Slug        string      `json:"slug"`
	Path        string      `json:"path"`       // content path; used by SkipPathRe
	AddedNice   string      `json:"added_nice"` // "YYYY-MM-DD"
	MemberViews stringOrInt `json:"member_views"`
	Preview     previewBlob `json:"preview_formatted"`
}

// previewBlob captures `preview_formatted.thumb.{ratio}[]` thumbnail
// signed URLs. Keys are ratio strings like `200-112`; each value is a
// slice with one item per CDN. We pick the largest ratio's first entry.
type previewBlob struct {
	Thumb map[string][]previewItem `json:"thumb"`
}

type previewItem struct {
	CMSContentServerID string `json:"cms_content_server_id"`
	FileURI            string `json:"fileuri"`
	Signature          string `json:"signature"`
}

// ---- Discovery + fetch ----

func (s *Scraper) fetchPageConfig(ctx context.Context) (*pageResponse, error) {
	u := s.cfg.NatsAPIBase + "/content/page?slug=/"
	body, err := s.fetchAPI(ctx, u)
	if err != nil {
		return nil, err
	}
	var resp pageResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode page config: %w", err)
	}
	if !resp.Success && resp.Error != "" {
		return nil, fmt.Errorf("page config: %s", resp.Error)
	}
	return &resp, nil
}

// findSetListBlockID walks a page's blocks and returns the first
// `set_list` block's CMSBlockID. Returns empty if no set_list block is
// present (which would indicate the home page doesn't host the videos
// list — the discovery would need to try a different slug instead).
func findSetListBlockID(page *pageResponse) string {
	for _, b := range page.Blocks {
		if b.Settings.Type == "set_list" {
			return b.CMSBlockID
		}
	}
	return ""
}

func (s *Scraper) fetchSets(ctx context.Context, blockID string) (*setsResponse, error) {
	u := s.cfg.NatsAPIBase + "/content/sets?cms_block_id=" + blockID
	body, err := s.fetchAPI(ctx, u)
	if err != nil {
		return nil, err
	}
	var resp setsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode sets: %w", err)
	}
	if !resp.Success && resp.Error != "" {
		return nil, fmt.Errorf("sets: %s", resp.Error)
	}
	return &resp, nil
}

// fetchServers returns the `cms_content_server_id → url` map needed to
// resolve thumbnail CDN hosts. Per-area — must be fetched after the
// area-id header is set.
func (s *Scraper) fetchServers(ctx context.Context) (map[string]string, error) {
	body, err := s.fetchAPI(ctx, s.cfg.NatsAPIBase+"/content/servers")
	if err != nil {
		return nil, err
	}
	var resp serversResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode servers: %w", err)
	}
	if !resp.Success && resp.Error != "" {
		return nil, fmt.Errorf("servers: %s", resp.Error)
	}
	m := make(map[string]string, len(resp.Servers))
	for _, sv := range resp.Servers {
		m[sv.CMSContentServerID] = strings.TrimRight(sv.Settings.URL, "/")
	}
	return m, nil
}

func (s *Scraper) fetchAPI(ctx context.Context, url string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: url,
		Headers: map[string]string{
			"Accept":             "application/json",
			"User-Agent":         httpx.UserAgentFirefox,
			"X-NATS-cms-area-id": s.cfg.CMSAreaID,
			"Referer":            s.cfg.SiteBase + "/",
			"Origin":             s.cfg.SiteBase,
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBodyN(resp.Body, maxSetsBytes)
}

// ---- run loop ----

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)
	scraper.Debugf(1, "natscms/%s: discovering set_list block via NATS CMS", s.cfg.ID)

	page, err := s.fetchPageConfig(ctx)
	if err != nil {
		select {
		case out <- scraper.Error(fmt.Errorf("page config: %w", err)):
		case <-ctx.Done():
		}
		return
	}
	blockID := findSetListBlockID(page)
	if blockID == "" {
		select {
		case out <- scraper.Error(fmt.Errorf("no set_list block on homepage for %s", s.cfg.ID)):
		case <-ctx.Done():
		}
		return
	}
	scraper.Debugf(1, "natscms/%s: set_list block_id=%s", s.cfg.ID, blockID)

	servers, err := s.fetchServers(ctx)
	if err != nil {
		// Don't bail — without servers we just skip thumbnails. Log and
		// keep going so a transient CDN-list failure doesn't lose a
		// whole scrape.
		scraper.Debugf(1, "natscms/%s: servers fetch failed (%v) — thumbnails omitted", s.cfg.ID, err)
		servers = nil
	} else {
		scraper.Debugf(1, "natscms/%s: %d CDN server(s)", s.cfg.ID, len(servers))
	}

	sets, err := s.fetchSets(ctx, blockID)
	if err != nil {
		select {
		case out <- scraper.Error(fmt.Errorf("fetch sets: %w", err)):
		case <-ctx.Done():
		}
		return
	}

	total := int(sets.TotalCount)
	if total == 0 {
		total = len(sets.Sets)
	}
	scraper.Debugf(1, "natscms/%s: %d total scenes", s.cfg.ID, total)
	if total > 0 {
		select {
		case out <- scraper.Progress(total):
		case <-ctx.Done():
			return
		}
	}

	now := time.Now().UTC()
	skipped := 0
	for _, entry := range sets.Sets {
		if s.cfg.SkipPathRe != nil && s.cfg.SkipPathRe.MatchString(entry.Path) {
			skipped++
			continue
		}
		if opts.KnownIDs[entry.CMSSetID] {
			scraper.Debugf(1, "natscms/%s: hit known ID %s, stopping early", s.cfg.ID, entry.CMSSetID)
			select {
			case out <- scraper.StoppedEarly():
			case <-ctx.Done():
			}
			return
		}
		select {
		case out <- scraper.Scene(s.toScene(entry, studioURL, servers, now)):
		case <-ctx.Done():
			return
		}
	}
	if skipped > 0 {
		scraper.Debugf(1, "natscms/%s: skipped %d non-video sets via SkipPathRe", s.cfg.ID, skipped)
	}
}

// ---- Scene materialisation ----

func (s *Scraper) toScene(e setEntry, studioURL string, servers map[string]string, now time.Time) models.Scene {
	scene := models.Scene{
		ID:          e.CMSSetID,
		SiteID:      s.cfg.ID,
		StudioURL:   studioURL,
		Title:       cleanHTML(e.Name),
		Description: cleanHTML(e.Description),
		// Trailer slug page — most sites route `/tour/trailer/{slug}` to
		// the SPA's scene-detail view.
		URL:       fmt.Sprintf("%s/tour/trailer/%s/", s.cfg.SiteBase, e.Slug),
		Studio:    s.cfg.StudioName,
		Series:    s.cfg.SiteName,
		ScrapedAt: now,
		Views:     int(e.MemberViews),
	}
	if d, err := time.Parse("2006-01-02", strings.TrimSpace(e.AddedNice)); err == nil {
		scene.Date = d.UTC()
	}
	scene.Thumbnail = pickThumbnail(e.Preview, servers)
	return scene
}

// pickThumbnail picks the highest-resolution preview from
// `preview_formatted.thumb.{ratio}[]` and resolves the CDN host via the
// `servers` map. Returns empty if no thumbnail or the CDN host is
// unknown. URL form: `{server}{fileuri}?{signature}` where signature is
// already a query string like `expires=…&token=…`.
func pickThumbnail(p previewBlob, servers map[string]string) string {
	if servers == nil {
		return ""
	}
	// Pick the largest ratio by width × height.
	var bestKey string
	var bestArea int
	for k := range p.Thumb {
		w, h, ok := parseRatio(k)
		if !ok {
			continue
		}
		if a := w * h; a > bestArea {
			bestArea = a
			bestKey = k
		}
	}
	if bestKey == "" {
		return ""
	}
	for _, it := range p.Thumb[bestKey] {
		base, ok := servers[it.CMSContentServerID]
		if !ok || base == "" || it.FileURI == "" {
			continue
		}
		url := base + it.FileURI
		if it.Signature != "" {
			url += "?" + it.Signature
		}
		return url
	}
	return ""
}

// parseRatio splits a ratio key like "200-112" into width/height ints.
func parseRatio(s string) (w, h int, ok bool) {
	i := strings.IndexByte(s, '-')
	if i < 0 {
		return 0, 0, false
	}
	var err1, err2 error
	w, err1 = strconv.Atoi(s[:i])
	h, err2 = strconv.Atoi(s[i+1:])
	return w, h, err1 == nil && err2 == nil
}

// ---- Helpers ----

var (
	htmlTagRe = regexp.MustCompile(`<[^>]+>`)
	wsRe      = regexp.MustCompile(`\s+`)
)

func cleanHTML(s string) string {
	if s == "" {
		return ""
	}
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, " ", " ") // U+00A0 from &nbsp;
	s = wsRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
