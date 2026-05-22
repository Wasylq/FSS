package vip4k

import (
	"context"
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
	"github.com/Wasylq/FSS/parseutil"
	"github.com/Wasylq/FSS/scraper"
)

const siteBase = "https://vip4k.com"

func init() { scraper.Register(New()) }

type Scraper struct {
	client *http.Client
}

func New() *Scraper {
	return &Scraper{
		client: httpx.NewClient(30 * time.Second),
	}
}

func (s *Scraper) ID() string { return "vip4k" }

func (s *Scraper) Patterns() []string {
	return []string{
		"vip4k.com",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?vip4k\.com(?:/|$)`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	cardStartRe = regexp.MustCompile(`<div class="item">`)
	idRe        = regexp.MustCompile(`href="/en/videos/(\d+)`)
	titleRe     = regexp.MustCompile(`<a class="item__title"[^>]*>([^<]+)</a>`)
	channelRe   = regexp.MustCompile(`<a class="item__site"[^>]*>([^<]+)</a>`)
	dateRe      = regexp.MustCompile(`<div class="item__date">(\d{4}-\d{2}-\d{2})</div>`)
	durRe       = regexp.MustCompile(`<div class="item__time">([^<]+)</div>`)
	thumbRe     = regexp.MustCompile(`<source srcset="([^"]+)" type="image/jpeg">`)
	previewRe   = regexp.MustCompile(`<source data-src="([^"]+)" type="video/mp4">`)
	perfRe      = regexp.MustCompile(`(?s)aria-label="([^"]+)">\s*<picture`)

	showMoreRe = regexp.MustCompile(`href="/en/publish/tag/all/all/all/(\d+)"`)

	detailDescRe = regexp.MustCompile(`(?s)<div class="player-description__text">(.*?)</div>`)
	detailTagRe  = regexp.MustCompile(`<a class="tags__item[^"]*"[^>]*>([^<]+)</a>`)
	detailPerfRe = regexp.MustCompile(`<div class="model__name">([^<]+)</div>`)
)

type listItem struct {
	id        string
	title     string
	channel   string
	date      time.Time
	duration  int
	thumbnail string
	preview   string
	performer string
}

type detailData struct {
	description string
	tags        []string
	performers  []string
}

func parseListingPage(body []byte) []listItem {
	locs := cardStartRe.FindAllIndex(body, -1)
	items := make([]listItem, 0, len(locs))
	seen := make(map[string]bool)
	for i, loc := range locs {
		end := len(body)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		card := body[loc[0]:end]
		if it, ok := parseCard(card); ok && !seen[it.id] {
			seen[it.id] = true
			items = append(items, it)
		}
	}
	return items
}

func parseCard(card []byte) (listItem, bool) {
	m := idRe.FindSubmatch(card)
	if m == nil {
		return listItem{}, false
	}

	it := listItem{id: string(m[1])}

	if mt := titleRe.FindSubmatch(card); mt != nil {
		it.title = html.UnescapeString(strings.TrimSpace(string(mt[1])))
	}

	if mc := channelRe.FindSubmatch(card); mc != nil {
		it.channel = html.UnescapeString(strings.TrimSpace(string(mc[1])))
	}

	if md := dateRe.FindSubmatch(card); md != nil {
		if t, err := time.Parse("2006-01-02", string(md[1])); err == nil {
			it.date = t
		}
	}

	if mDur := durRe.FindSubmatch(card); mDur != nil {
		it.duration = parseutil.ParseDurationColon(strings.TrimSpace(string(mDur[1])))
	}

	if mThumb := thumbRe.FindSubmatch(card); mThumb != nil {
		it.thumbnail = ensureHTTPS(string(mThumb[1]))
	}

	if mPrev := previewRe.FindSubmatch(card); mPrev != nil {
		it.preview = ensureHTTPS(string(mPrev[1]))
	}

	if mPerf := perfRe.FindSubmatch(card); mPerf != nil {
		it.performer = html.UnescapeString(strings.TrimSpace(string(mPerf[1])))
	}

	return it, true
}

func parseDetailPage(body []byte) detailData {
	var d detailData

	if m := detailDescRe.FindSubmatch(body); m != nil {
		desc := strings.TrimSpace(string(m[1]))
		desc = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(desc, "")
		d.description = html.UnescapeString(strings.TrimSpace(desc))
	}

	seen := make(map[string]bool)
	for _, m := range detailTagRe.FindAllSubmatch(body, -1) {
		tag := html.UnescapeString(strings.TrimSpace(string(m[1])))
		if tag != "" && !seen[tag] {
			seen[tag] = true
			d.tags = append(d.tags, tag)
		}
	}

	for _, m := range detailPerfRe.FindAllSubmatch(body, -1) {
		name := html.UnescapeString(strings.TrimSpace(string(m[1])))
		if name != "" {
			d.performers = append(d.performers, name)
		}
	}

	return d
}

func hasNextPage(body []byte) bool {
	return showMoreRe.Match(body)
}

func ensureHTTPS(url string) string {
	if strings.HasPrefix(url, "//") {
		return "https:" + url
	}
	return url
}

func channelToSiteID(channel string) string {
	s := strings.ToLower(strings.TrimSpace(channel))
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)
	s.runWithBase(ctx, siteBase, studioURL, opts, out)
}

func (s *Scraper) runWithBase(ctx context.Context, base, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	for page := 1; ; page++ {
		if ctx.Err() != nil {
			return
		}
		if page > 1 && opts.Delay > 0 {
			select {
			case <-time.After(opts.Delay):
			case <-ctx.Done():
				return
			}
		}

		scraper.Debugf(1, "vip4k: fetching page %d", page)

		pageURL := fmt.Sprintf("%s/en/publish/tag/all/all/all/%d", base, page)
		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		items := parseListingPage(body)
		if len(items) == 0 {
			return
		}

		if page == 1 {
			total := estimateTotal(body, len(items))
			scraper.Debugf(1, "vip4k: %d total scenes (estimated)", total)
			if total > 0 {
				select {
				case out <- scraper.Progress(total):
				case <-ctx.Done():
					return
				}
			}
		}

		stopped := s.fetchDetailsAndSend(ctx, items, base, studioURL, opts, out)
		if stopped {
			return
		}

		if !hasNextPage(body) {
			return
		}
	}
}

func estimateTotal(body []byte, perPage int) int {
	maxPage := 1
	for _, m := range showMoreRe.FindAllSubmatch(body, -1) {
		n, _ := strconv.Atoi(string(m[1]))
		if n > maxPage {
			maxPage = n
		}
	}
	if maxPage <= 1 {
		return perPage
	}
	return maxPage * perPage
}

func (s *Scraper) fetchDetailsAndSend(ctx context.Context, items []listItem, base, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) (stopped bool) {
	workers := opts.Workers
	if workers <= 0 {
		workers = 4
	}

	scraper.Debugf(1, "vip4k: fetching %d details with %d workers", len(items), workers)

	type enriched struct {
		item   listItem
		detail detailData
		err    error
	}

	results := make([]enriched, len(items))
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)

	for i, it := range items {
		if ctx.Err() != nil {
			return false
		}
		wg.Add(1)
		go func(idx int, item listItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if opts.Delay > 0 {
				select {
				case <-time.After(opts.Delay):
				case <-ctx.Done():
					return
				}
			}

			detail, err := s.fetchDetail(ctx, base, item.id)
			results[idx] = enriched{item: item, detail: detail, err: err}
		}(i, it)
	}
	wg.Wait()

	now := time.Now().UTC()
	for _, r := range results {
		if ctx.Err() != nil {
			return false
		}
		if r.err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("detail %s: %w", r.item.id, r.err)):
			case <-ctx.Done():
			}
			continue
		}

		if opts.KnownIDs[r.item.id] {
			scraper.Debugf(1, "vip4k: hit known ID, stopping early")
			select {
			case out <- scraper.StoppedEarly():
			case <-ctx.Done():
			}
			return true
		}

		scene := toScene(r.item, r.detail, studioURL, now)
		select {
		case out <- scraper.Scene(scene):
		case <-ctx.Done():
			return false
		}
	}
	return false
}

func (s *Scraper) fetchDetail(ctx context.Context, base, videoID string) (detailData, error) {
	url := fmt.Sprintf("%s/en/videos/%s", base, videoID)
	body, err := s.fetchPage(ctx, url)
	if err != nil {
		return detailData{}, err
	}
	return parseDetailPage(body), nil
}

func toScene(it listItem, d detailData, studioURL string, now time.Time) models.Scene {
	siteID := "vip4k"
	studio := "VIP 4K"
	if it.channel != "" {
		siteID = channelToSiteID(it.channel)
		studio = it.channel
	}

	performers := d.performers
	if len(performers) == 0 && it.performer != "" {
		performers = []string{it.performer}
	}

	return models.Scene{
		ID:          it.id,
		SiteID:      siteID,
		StudioURL:   studioURL,
		Title:       it.title,
		URL:         siteBase + "/en/videos/" + it.id,
		Thumbnail:   it.thumbnail,
		Preview:     it.preview,
		Date:        it.date,
		Duration:    it.duration,
		Performers:  performers,
		Description: d.description,
		Tags:        d.tags,
		Studio:      studio,
		ScrapedAt:   now,
	}
}

func (s *Scraper) fetchPage(ctx context.Context, url string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     url,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}
