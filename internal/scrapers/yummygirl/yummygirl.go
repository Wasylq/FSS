package yummygirl

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
	"github.com/Wasylq/FSS/scraper"
)

const defaultSiteBase = "https://yummygirl.com"

type urlKind int

const (
	kindUpdates urlKind = iota
	kindModel
)

type Scraper struct {
	client   *http.Client
	siteBase string
}

func New() *Scraper {
	return &Scraper{
		client:   httpx.NewClient(30 * time.Second),
		siteBase: defaultSiteBase,
	}
}

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return "yummygirl" }

func (s *Scraper) Patterns() []string {
	return []string{
		"yummygirl.com",
		"yummygirl.com/models/{slug}.html",
	}
}

var (
	matchRe = regexp.MustCompile(`^https?://(?:www\.)?yummygirl\.com`)
	modelRe = regexp.MustCompile(`/models/([^/?#]+)\.html`)
)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func classifyURL(u string) urlKind {
	if m := modelRe.FindStringSubmatch(u); m != nil && m[1] != "models" {
		return kindModel
	}
	return kindUpdates
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	switch classifyURL(studioURL) {
	case kindModel:
		s.runModel(ctx, studioURL, opts, out)
	default:
		s.runUpdates(ctx, studioURL, opts, out)
	}
}

func (s *Scraper) runUpdates(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	now := time.Now().UTC()

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

		pageURL := fmt.Sprintf("%s/categories/movies_%d_d.html", s.siteBase, page)

		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		scenes := parseListingCards(body)
		if len(scenes) == 0 {
			return
		}

		if page == 1 {
			total := estimateTotal(body, len(scenes))
			if total > 0 {
				select {
				case out <- scraper.Progress(total):
				case <-ctx.Done():
					return
				}
			}
		}

		if s.emitScenes(ctx, scenes, studioURL, now, opts, out) {
			return
		}

		if !hasNextPage(body, page) {
			return
		}
	}
}

func (s *Scraper) runModel(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	now := time.Now().UTC()

	body, err := s.fetchPage(ctx, studioURL)
	if err != nil {
		select {
		case out <- scraper.Error(fmt.Errorf("model page: %w", err)):
		case <-ctx.Done():
		}
		return
	}

	modelID, maxPage := extractModelPagination(body)

	scenes := parseModelBlocks(body)
	if len(scenes) == 0 {
		return
	}

	if maxPage > 0 {
		select {
		case out <- scraper.Progress(len(scenes) * maxPage):
		case <-ctx.Done():
			return
		}
	}

	if s.emitScenes(ctx, scenes, studioURL, now, opts, out) {
		return
	}

	if modelID == "" || maxPage <= 1 {
		return
	}

	for page := 2; page <= maxPage; page++ {
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

		pageURL := fmt.Sprintf("%s/sets.php?id=%s&page=%d", s.siteBase, modelID, page)
		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("model page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		scenes := parseModelBlocks(body)
		if len(scenes) == 0 {
			return
		}

		if s.emitScenes(ctx, scenes, studioURL, now, opts, out) {
			return
		}
	}
}

func (s *Scraper) emitScenes(ctx context.Context, scenes []parsedScene, studioURL string, now time.Time, opts scraper.ListOpts, out chan<- scraper.SceneResult) (stopped bool) {
	for _, ps := range scenes {
		if opts.KnownIDs[ps.id] {
			select {
			case out <- scraper.StoppedEarly():
			case <-ctx.Done():
			}
			return true
		}

		scene := toScene(ps, s.siteBase, studioURL, now)
		select {
		case out <- scraper.Scene(scene):
		case <-ctx.Done():
			return true
		}
	}
	return false
}

func (s *Scraper) fetchPage(ctx context.Context, url string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: url,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentFirefox,
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}

type parsedScene struct {
	id          string
	title       string
	relURL      string
	thumbnail   string
	performers  []string
	date        string
	description string
	tags        []string
}

var (
	cardStartRe    = regexp.MustCompile(`<div class="updateItem">`)
	sceneSlugRe    = regexp.MustCompile(`<a\s+href="[^"]*?/updates/([^"]+)\.html"`)
	listTitleRe    = regexp.MustCompile(`(?s)<h4>\s*<a[^>]+>([^<]+)</a>`)
	thumbRe        = regexp.MustCompile(`src0_1x="([^"]+)"`)
	performerRe    = regexp.MustCompile(`<a\s+href="[^"]*?/models/[^"]+">([^<]+)</a>`)
	listDateRe     = regexp.MustCompile(`<span>(\d{2}/\d{2}/\d{4})</span>`)
	paginationRe   = regexp.MustCompile(`(?s)<div\s+class="global_pagination[^"]*">(.*?)</div>`)
	maxPageRe      = regexp.MustCompile(`movies_(\d+)(?:_d)?\.html`)
	modelIDRe      = regexp.MustCompile(`sets\.php\?id=(\d+)`)
	modelMaxPageRe = regexp.MustCompile(`(?:movies_|page=)(\d+)`)

	blockStartRe  = regexp.MustCompile(`<div class="update_block">`)
	blockTitleRe  = regexp.MustCompile(`<span class="update_title">([^<]+)</span>`)
	blockDateRe   = regexp.MustCompile(`<span class="availdate">(\d{2}/\d{2}/\d{4})`)
	blockDescRe   = regexp.MustCompile(`(?s)<span class="latest_update_description">(.*?)</span>`)
	blockTagLinkRe = regexp.MustCompile(`<a\s+href="[^"]*?/categories/[^"]+">([^<]+)</a>`)
	blockTagsRe   = regexp.MustCompile(`(?s)<span class="update_tags">(.*?)</span>`)
)

func parseListingCards(body []byte) []parsedScene {
	page := string(body)
	starts := cardStartRe.FindAllStringIndex(page, -1)
	seen := make(map[string]bool, len(starts))
	scenes := make([]parsedScene, 0, len(starts))

	for i, loc := range starts {
		end := len(page)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		block := page[loc[0]:end]

		var ps parsedScene

		if m := sceneSlugRe.FindStringSubmatch(block); m != nil {
			ps.id = m[1]
			ps.relURL = "/updates/" + m[1] + ".html"
		}
		if ps.id == "" || seen[ps.id] {
			continue
		}
		seen[ps.id] = true

		if m := listTitleRe.FindStringSubmatch(block); m != nil {
			ps.title = html.UnescapeString(strings.TrimSpace(m[1]))
		}

		if m := thumbRe.FindStringSubmatch(block); m != nil {
			ps.thumbnail = m[1]
		}

		for _, m := range performerRe.FindAllStringSubmatch(block, -1) {
			name := strings.TrimSpace(m[1])
			if name != "" {
				ps.performers = append(ps.performers, name)
			}
		}

		if m := listDateRe.FindStringSubmatch(block); m != nil {
			ps.date = m[1]
		}

		scenes = append(scenes, ps)
	}
	return scenes
}

func parseModelBlocks(body []byte) []parsedScene {
	page := string(body)
	starts := blockStartRe.FindAllStringIndex(page, -1)
	seen := make(map[string]bool, len(starts))
	scenes := make([]parsedScene, 0, len(starts))

	for i, loc := range starts {
		end := len(page)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		block := page[loc[0]:end]

		var ps parsedScene

		if m := sceneSlugRe.FindStringSubmatch(block); m != nil {
			ps.id = m[1]
			ps.relURL = "/updates/" + m[1] + ".html"
		}
		if ps.id == "" || seen[ps.id] {
			continue
		}
		seen[ps.id] = true

		if m := blockTitleRe.FindStringSubmatch(block); m != nil {
			ps.title = html.UnescapeString(strings.TrimSpace(m[1]))
		}

		if m := thumbRe.FindStringSubmatch(block); m != nil {
			ps.thumbnail = m[1]
		}

		for _, m := range performerRe.FindAllStringSubmatch(block, -1) {
			name := strings.TrimSpace(m[1])
			if name != "" {
				ps.performers = append(ps.performers, name)
			}
		}

		if m := blockDateRe.FindStringSubmatch(block); m != nil {
			ps.date = m[1]
		}

		if m := blockDescRe.FindStringSubmatch(block); m != nil {
			ps.description = strings.TrimSpace(html.UnescapeString(m[1]))
		}

		if tm := blockTagsRe.FindStringSubmatch(block); tm != nil {
			for _, m := range blockTagLinkRe.FindAllStringSubmatch(tm[1], -1) {
				tag := strings.TrimSpace(m[1])
				if tag != "" {
					ps.tags = append(ps.tags, tag)
				}
			}
		}

		scenes = append(scenes, ps)
	}
	return scenes
}

func estimateTotal(body []byte, firstPageCount int) int {
	pm := paginationRe.FindSubmatch(body)
	if pm == nil {
		return firstPageCount
	}

	maxPage := 1
	for _, m := range maxPageRe.FindAllSubmatch(pm[1], -1) {
		if n, _ := strconv.Atoi(string(m[1])); n > maxPage {
			maxPage = n
		}
	}
	return maxPage * firstPageCount
}

func hasNextPage(body []byte, currentPage int) bool {
	pm := paginationRe.FindSubmatch(body)
	if pm == nil {
		return false
	}

	maxPage := 1
	for _, m := range maxPageRe.FindAllSubmatch(pm[1], -1) {
		if n, _ := strconv.Atoi(string(m[1])); n > maxPage {
			maxPage = n
		}
	}
	return currentPage < maxPage
}

func extractModelPagination(body []byte) (modelID string, maxPage int) {
	pm := paginationRe.FindSubmatch(body)
	if pm == nil {
		return "", 0
	}

	if m := modelIDRe.FindSubmatch(pm[1]); m != nil {
		modelID = string(m[1])
	}

	maxPage = 1
	for _, m := range modelMaxPageRe.FindAllSubmatch(pm[1], -1) {
		if n, _ := strconv.Atoi(string(m[1])); n > maxPage {
			maxPage = n
		}
	}
	return modelID, maxPage
}

func toScene(ps parsedScene, siteBase, studioURL string, now time.Time) models.Scene {
	sceneURL := ps.relURL
	if sceneURL != "" && !strings.HasPrefix(sceneURL, "http") {
		sceneURL = siteBase + "/" + strings.TrimPrefix(sceneURL, "/")
	}

	thumb := ps.thumbnail
	if thumb != "" && !strings.HasPrefix(thumb, "http") {
		if !strings.HasPrefix(thumb, "/") {
			thumb = "/" + thumb
		}
		thumb = siteBase + thumb
	}

	scene := models.Scene{
		ID:          ps.id,
		SiteID:      "yummygirl",
		StudioURL:   studioURL,
		Title:       ps.title,
		URL:         sceneURL,
		Thumbnail:   thumb,
		Performers:  ps.performers,
		Date:        parseDate(ps.date),
		Description: ps.description,
		Tags:        ps.tags,
		Studio:      "YummyGirl",
		ScrapedAt:   now,
	}
	return scene
}

func parseDate(s string) time.Time {
	t, err := time.Parse("01/02/2006", s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}
