package kink

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

type Scraper struct {
	client *http.Client
}

func New() *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second)}
}

func init() {
	scraper.Register(New())
}

func (s *Scraper) ID() string { return "kink" }

func (s *Scraper) Patterns() []string {
	return []string{
		"kink.com",
		"kink.com/channel/{slug}",
		"kink.com/model/{id}/{slug}",
		"kink.com/tag/{slug}",
		"kink.com/series/{slug}",
		"kink.com/shoots?channelIds={slug}",
		"kink.com/shoots?performerIds={id}",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?kink\.com`)

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	if opts.Workers <= 0 {
		opts.Workers = 3
	}
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

type listEntry struct {
	id         string
	title      string
	url        string
	thumbnail  string
	preview    string
	performers []string
	channel    string
	date       string
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	lc := resolveListingConfig(studioURL)
	if lc.mode == modeSeries {
		s.runSeries(ctx, lc, opts, out)
		return
	}

	work := make(chan listEntry, opts.Workers)
	var wg sync.WaitGroup

	for i := 0; i < opts.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for entry := range work {
				if opts.Delay > 0 {
					select {
					case <-time.After(opts.Delay):
					case <-ctx.Done():
						return
					}
				}
				scene, err := s.fetchDetail(ctx, entry)
				if err != nil {
					select {
					case out <- scraper.Error(err):
					case <-ctx.Done():
						return
					}
					continue
				}
				select {
				case out <- scraper.Scene(scene):
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	sentTotal := false
	for page := 1; ; page++ {
		if ctx.Err() != nil {
			break
		}
		if page > 1 && opts.Delay > 0 {
			select {
			case <-time.After(opts.Delay):
			case <-ctx.Done():
				break
			}
			if ctx.Err() != nil {
				break
			}
		}

		pageURL := lc.pageURL(page)
		entries, totalPages, err := s.fetchListing(ctx, pageURL, lc.baseURL)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			break
		}

		if len(entries) == 0 {
			break
		}

		if !sentTotal && totalPages > 0 {
			sentTotal = true
			select {
			case out <- scraper.Progress(totalPages * 24):
			case <-ctx.Done():
			}
		}

		cancelled := false
		hitKnown := false
		for _, e := range entries {
			if len(opts.KnownIDs) > 0 && opts.KnownIDs[e.id] {
				hitKnown = true
				break
			}
			select {
			case work <- e:
			case <-ctx.Done():
				cancelled = true
			}
			if cancelled {
				break
			}
		}
		if cancelled || hitKnown {
			if hitKnown {
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
			}
			break
		}

		if page >= totalPages {
			break
		}
	}

	close(work)
	wg.Wait()
}

var seriesShootIDRe = regexp.MustCompile(`imagedb/(\d+)/`)

func (s *Scraper) runSeries(ctx context.Context, lc listingConfig, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	body, err := s.fetchHTML(ctx, lc.baseURL)
	if err != nil {
		select {
		case out <- scraper.Error(fmt.Errorf("series page: %w", err)):
		case <-ctx.Done():
		}
		return
	}

	base := "https://www.kink.com"
	if m := baseHostRe.FindString(lc.baseURL); m != "" {
		base = m
	}

	seen := make(map[string]bool)
	var ids []string
	for _, m := range seriesShootIDRe.FindAllSubmatch(body, -1) {
		id := string(m[1])
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}

	if len(ids) > 0 {
		select {
		case out <- scraper.Progress(len(ids)):
		case <-ctx.Done():
			return
		}
	}

	for i, id := range ids {
		if ctx.Err() != nil {
			return
		}
		if len(opts.KnownIDs) > 0 && opts.KnownIDs[id] {
			select {
			case out <- scraper.StoppedEarly():
			case <-ctx.Done():
			}
			return
		}
		if i > 0 && opts.Delay > 0 {
			select {
			case <-time.After(opts.Delay):
			case <-ctx.Done():
				return
			}
		}
		entry := listEntry{
			id:  id,
			url: fmt.Sprintf("%s/shoot/%s", base, id),
		}
		scene, fetchErr := s.fetchDetail(ctx, entry)
		if fetchErr != nil {
			select {
			case out <- scraper.Error(fetchErr):
			case <-ctx.Done():
				return
			}
			continue
		}
		select {
		case out <- scraper.Scene(scene):
		case <-ctx.Done():
			return
		}
	}
}

var (
	channelPathRe = regexp.MustCompile(`/channel/([^/?#]+)`)
	modelPathRe   = regexp.MustCompile(`/model/(\d+)`)
	tagPathRe     = regexp.MustCompile(`/tag/([^/?#]+)`)
	seriesPathRe  = regexp.MustCompile(`/series/([^/?#]+)`)
	baseHostRe    = regexp.MustCompile(`^(https?://[^/]+)`)
)

type listingMode int

const (
	modeShootsAPI  listingMode = iota // /shoots?...&page=N
	modeDirectPage                    // /tag/{slug}?page=N or /channel/{slug}?page=N
	modeSeries                        // /series/{slug} — extract IDs from single page
)

type listingConfig struct {
	mode    listingMode
	baseURL string // full URL without page param
}

func resolveListingConfig(rawURL string) listingConfig {
	base := "https://www.kink.com"
	if m := baseHostRe.FindString(rawURL); m != "" {
		base = m
	}
	if m := seriesPathRe.FindStringSubmatch(rawURL); m != nil {
		return listingConfig{mode: modeSeries, baseURL: base + "/series/" + m[1]}
	}
	if m := tagPathRe.FindStringSubmatch(rawURL); m != nil {
		return listingConfig{mode: modeDirectPage, baseURL: base + "/tag/" + m[1]}
	}
	if m := channelPathRe.FindStringSubmatch(rawURL); m != nil {
		slug := strings.ReplaceAll(m[1], "-", "")
		return listingConfig{mode: modeShootsAPI, baseURL: base + "/shoots?channelIds=" + slug + "&sort=published"}
	}
	if m := modelPathRe.FindStringSubmatch(rawURL); m != nil {
		return listingConfig{mode: modeShootsAPI, baseURL: base + "/shoots?performerIds=" + m[1] + "&sort=published"}
	}
	if strings.Contains(rawURL, "channelIds=") || strings.Contains(rawURL, "performerIds=") || strings.Contains(rawURL, "tagIds=") {
		u := rawURL
		if !strings.Contains(u, "sort=") {
			u += "&sort=published"
		}
		return listingConfig{mode: modeShootsAPI, baseURL: u}
	}
	return listingConfig{mode: modeShootsAPI, baseURL: base + "/shoots?sort=published"}
}

func (lc listingConfig) pageURL(page int) string {
	switch lc.mode {
	case modeDirectPage:
		if strings.Contains(lc.baseURL, "?") {
			return fmt.Sprintf("%s&page=%d", lc.baseURL, page)
		}
		return fmt.Sprintf("%s?page=%d", lc.baseURL, page)
	default:
		return fmt.Sprintf("%s&page=%d", lc.baseURL, page)
	}
}

var (
	shootCardRe = regexp.MustCompile(`(?s)<div class="card shoot-thumbnail[^"]*">.*?</div>\s*</div>\s*</div>`)
	shootLinkRe = regexp.MustCompile(`href="/shoot/(\d+)"`)
	titleAttrRe = regexp.MustCompile(`title="([^"]+)"[^>]*class="d-block overflow-hidden text-elipsis h5"`)
	titleTextRe = regexp.MustCompile(`class="d-block overflow-hidden text-elipsis h5">\s*([^<]+?)\s*</a>`)
	imgSrcRe    = regexp.MustCompile(`data-src="([^"]+)"[^>]*alt="Video shoot"`)
	trailerRe   = regexp.MustCompile(`data-trailer-url="([^"]+)"`)
	modelLnkRe  = regexp.MustCompile(`href="/model/\d+/[^"]*"[^>]*>([^<]+)</a>`)
	channelRe   = regexp.MustCompile(`href="/channel/([^"]+)"[^>]*class="channel-tag[^"]*">\s*<small>\s*([^<]+?)\s*</small>`)
	dateSpanRe  = regexp.MustCompile(`<span class="no-blur">([A-Z][a-z]{2} \d{1,2}, \d{4})</span>`)
	lastPageRe  = regexp.MustCompile(`data-page="(\d+)">\d+</div>\s*</li>\s*<li class="page-item">\s*<div[^>]*>(?:>>|&gt;&gt;)`)
)

func (s *Scraper) fetchListing(ctx context.Context, pageURL, listBase string) ([]listEntry, int, error) {
	base := "https://www.kink.com"
	if m := baseHostRe.FindString(listBase); m != "" {
		base = m
	}
	body, err := s.fetchHTML(ctx, pageURL)
	if err != nil {
		return nil, 0, err
	}

	totalPages := 0
	if m := lastPageRe.FindSubmatch(body); m != nil {
		totalPages, _ = strconv.Atoi(string(m[1]))
	}

	cards := shootCardRe.FindAll(body, -1)
	entries := make([]listEntry, 0, len(cards))
	seen := make(map[string]bool)

	for _, card := range cards {
		sm := shootLinkRe.FindSubmatch(card)
		if sm == nil {
			continue
		}
		id := string(sm[1])
		if seen[id] {
			continue
		}
		seen[id] = true

		e := listEntry{
			id:  id,
			url: fmt.Sprintf("%s/shoot/%s", base, id),
		}

		if m := titleAttrRe.FindSubmatch(card); m != nil {
			e.title = html.UnescapeString(string(m[1]))
		} else if m := titleTextRe.FindSubmatch(card); m != nil {
			e.title = strings.TrimSpace(html.UnescapeString(string(m[1])))
		}

		if m := imgSrcRe.FindSubmatch(card); m != nil {
			e.thumbnail = html.UnescapeString(string(m[1]))
		}

		if m := trailerRe.FindSubmatch(card); m != nil {
			e.preview = html.UnescapeString(string(m[1]))
		}

		for _, m := range modelLnkRe.FindAllSubmatch(card, -1) {
			e.performers = append(e.performers, strings.TrimSpace(html.UnescapeString(string(m[1]))))
		}

		if m := channelRe.FindSubmatch(card); m != nil {
			e.channel = strings.TrimSpace(html.UnescapeString(string(m[2])))
		}

		if m := dateSpanRe.FindSubmatch(card); m != nil {
			e.date = string(m[1])
		}

		entries = append(entries, e)
	}

	return entries, totalPages, nil
}

type jsonLD struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	UploadDate  string    `json:"uploadDate"`
	Thumbnail   string    `json:"thumbnailUrl"`
	ContentURL  string    `json:"contentUrl"`
	Duration    string    `json:"duration"`
	Actors      []ldActor `json:"actor"`
	Director    *ldActor  `json:"director"`
}

type ldActor struct {
	Name string `json:"name"`
}

type dataSetup struct {
	Duration    int             `json:"duration"`
	ChannelName string          `json:"channelName"`
	Resolutions map[string]bool `json:"resolutions"`
	Tracking    trackingData    `json:"trackingData"`
}

type trackingData struct {
	TagIDs     []string `json:"tagIds"`
	ModelNames []string `json:"modelNames"`
}

var (
	jsonLDRe    = regexp.MustCompile(`<script type="application/ld\+json">\s*(\{[^}]*"@type"\s*:\s*"VideoObject"[^<]*)</script>`)
	dataSetupRe = regexp.MustCompile(`data-setup="([^"]+)"`)
)

func (s *Scraper) fetchDetail(ctx context.Context, entry listEntry) (models.Scene, error) {
	body, err := s.fetchHTML(ctx, entry.url)
	if err != nil {
		return models.Scene{}, fmt.Errorf("detail %s: %w", entry.id, err)
	}

	now := time.Now().UTC()
	scene := models.Scene{
		ID:         entry.id,
		SiteID:     "kink",
		StudioURL:  "https://www.kink.com",
		Title:      entry.title,
		URL:        entry.url,
		Thumbnail:  entry.thumbnail,
		Preview:    entry.preview,
		Performers: entry.performers,
		Studio:     entry.channel,
		ScrapedAt:  now,
	}

	if entry.date != "" {
		if t, err := time.Parse("Jan 2, 2006", entry.date); err == nil {
			scene.Date = t.UTC()
		}
	}

	if m := jsonLDRe.FindSubmatch(body); m != nil {
		var ld jsonLD
		if err := json.Unmarshal(m[1], &ld); err == nil {
			if ld.Description != "" {
				scene.Description = ld.Description
			}
			if ld.Thumbnail != "" {
				scene.Thumbnail = ld.Thumbnail
			}
			if ld.ContentURL != "" && scene.Preview == "" {
				scene.Preview = ld.ContentURL
			}
			if len(scene.Performers) == 0 {
				for _, a := range ld.Actors {
					scene.Performers = append(scene.Performers, a.Name)
				}
			}
		}
	}

	if m := dataSetupRe.FindSubmatch(body); m != nil {
		decoded := html.UnescapeString(string(m[1]))
		var ds dataSetup
		if err := json.Unmarshal([]byte(decoded), &ds); err == nil {
			if ds.Duration > 0 {
				scene.Duration = ds.Duration / 1000
			}
			for _, tag := range ds.Tracking.TagIDs {
				tag = strings.ReplaceAll(tag, "-", " ")
				if tag != "" {
					scene.Tags = append(scene.Tags, tag)
				}
			}
			if len(scene.Performers) == 0 {
				scene.Performers = ds.Tracking.ModelNames
			}
			if ds.Resolutions["2160p"] {
				scene.Tags = appendIfMissing(scene.Tags, "4K")
			}
		}
	}

	return scene, nil
}

func (s *Scraper) fetchHTML(ctx context.Context, rawURL string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: rawURL,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentFirefox,
			"Cookie":     "age_gate_accepted=1",
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}

func appendIfMissing(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}
