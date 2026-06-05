// Package chickpass scrapes the ChickPass Network sites that run on the NATS
// CMS (chickpasscash.com). Each sub-site has its own cms-area-id and a filtered
// content pool. The hub (chickpassnetwork.com) returns the full catalogue.
//
// The NATS CMS pattern is identical to the aziani and marsmedia packages —
// the only differences are the API host, enhanced data_types query for
// performers/tags, and the studio name.
package chickpass

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

const (
	natsAPIBase = "https://www.chickpasscash.com/tour_api.php"
	studioName  = "ChickPass"
)

type SiteConfig struct {
	ID        string
	SiteBase  string
	SiteName  string
	CMSAreaID string
	Patterns  []string
	MatchRe   *regexp.Regexp
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
}

type serverEntry struct {
	CMSContentServerID string         `json:"cms_content_server_id"`
	Settings           serverSettings `json:"settings"`
}

type serverSettings struct {
	URL string `json:"url"`
}

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
	AddedNice   string      `json:"added_nice"`
	MemberViews stringOrInt `json:"member_views"`
	Preview     previewBlob `json:"preview_formatted"`
	DataTypes   []dataType  `json:"data_types"`
}

type dataType struct {
	Type   string      `json:"data_type"`
	Values []dataValue `json:"data_values"`
}

type dataValue struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

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
	u := natsAPIBase + "/content/page?slug=/"
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

func findSetListBlockID(page *pageResponse) string {
	for _, b := range page.Blocks {
		if b.Settings.Type == "set_list" {
			return b.CMSBlockID
		}
	}
	return ""
}

func (s *Scraper) fetchSets(ctx context.Context, blockID string) (*setsResponse, error) {
	u := natsAPIBase + "/content/sets?cms_block_id=" + blockID + "&data_types=1&content_count=1"
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

func (s *Scraper) fetchServers(ctx context.Context) (map[string]string, error) {
	body, err := s.fetchAPI(ctx, natsAPIBase+"/content/servers")
	if err != nil {
		return nil, err
	}
	var resp serversResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode servers: %w", err)
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
	return httpx.ReadBody(resp.Body)
}

// ---- run loop ----

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)
	scraper.Debugf(1, "chickpass/%s: discovering set_list block via NATS CMS", s.cfg.ID)

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
	scraper.Debugf(1, "chickpass/%s: set_list block_id=%s", s.cfg.ID, blockID)

	servers, err := s.fetchServers(ctx)
	if err != nil {
		scraper.Debugf(1, "chickpass/%s: servers fetch failed (%v) — thumbnails omitted", s.cfg.ID, err)
		servers = nil
	} else {
		scraper.Debugf(1, "chickpass/%s: %d CDN server(s)", s.cfg.ID, len(servers))
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
	scraper.Debugf(1, "chickpass/%s: %d total scenes", s.cfg.ID, total)
	if total > 0 {
		select {
		case out <- scraper.Progress(total):
		case <-ctx.Done():
			return
		}
	}

	now := time.Now().UTC()
	for _, entry := range sets.Sets {
		if opts.KnownIDs[entry.CMSSetID] {
			scraper.Debugf(1, "chickpass/%s: hit known ID %s, stopping early", s.cfg.ID, entry.CMSSetID)
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
}

// ---- Scene materialisation ----

func (s *Scraper) toScene(e setEntry, studioURL string, servers map[string]string, now time.Time) models.Scene {
	scene := models.Scene{
		ID:          e.CMSSetID,
		SiteID:      s.cfg.ID,
		StudioURL:   studioURL,
		Title:       cleanHTML(e.Name),
		Description: cleanHTML(e.Description),
		URL:         fmt.Sprintf("%s/video/%s", s.cfg.SiteBase, e.Slug),
		Studio:      studioName,
		Series:      s.cfg.SiteName,
		ScrapedAt:   now,
		Views:       int(e.MemberViews),
	}
	if d, err := time.Parse("2006-01-02", strings.TrimSpace(e.AddedNice)); err == nil {
		scene.Date = d.UTC()
	}
	scene.Thumbnail = pickThumbnail(e.Preview, servers)
	scene.Performers, scene.Tags = extractDataTypes(e.DataTypes)
	return scene
}

func extractDataTypes(dts []dataType) (performers, tags []string) {
	for _, dt := range dts {
		switch dt.Type {
		case "Models":
			for _, v := range dt.Values {
				if v.Name != "" {
					performers = append(performers, v.Name)
				}
			}
		case "Category":
			for _, v := range dt.Values {
				if v.Name != "" {
					tags = append(tags, v.Name)
				}
			}
		}
	}
	return performers, tags
}

func pickThumbnail(p previewBlob, servers map[string]string) string {
	if servers == nil {
		return ""
	}
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
	s = strings.ReplaceAll(s, " ", " ")
	s = wsRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
