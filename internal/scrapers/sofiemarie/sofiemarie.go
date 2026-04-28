package sofiemarie

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const defaultSiteBase = "https://sofiemariexxx.com"

type urlKind int

const (
	kindUpdates urlKind = iota
	kindModel
	kindDVD
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

func (s *Scraper) ID() string { return "sofiemarie" }

func (s *Scraper) Patterns() []string {
	return []string{
		"sofiemariexxx.com",
		"sofiemariexxx.com/models/{slug}.html",
		"sofiemariexxx.com/dvds/{slug}.html",
	}
}

var (
	matchRe = regexp.MustCompile(`^https?://(?:www\.)?sofiemariexxx\.com`)
	modelRe = regexp.MustCompile(`/models/([^/?#]+)\.html`)
	dvdRe   = regexp.MustCompile(`/dvds/([^/?#]+)\.html`)
)

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func classifyURL(u string) urlKind {
	if modelRe.MatchString(u) {
		return kindModel
	}
	if dvdRe.MatchString(u) {
		return kindDVD
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
	case kindDVD:
		s.runDVD(ctx, studioURL, opts, out)
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

		var pageURL string
		if page == 1 {
			pageURL = s.siteBase + "/categories/movies.html"
		} else {
			pageURL = fmt.Sprintf("%s/categories/movies_%d.html", s.siteBase, page)
		}

		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		scenes := parseSceneBlocks(body)
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

	scenes := parseSceneBlocks(body)
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

		scenes := parseSceneBlocks(body)
		if len(scenes) == 0 {
			return
		}

		if s.emitScenes(ctx, scenes, studioURL, now, opts, out) {
			return
		}
	}
}

func (s *Scraper) runDVD(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	now := time.Now().UTC()

	body, err := s.fetchPage(ctx, studioURL)
	if err != nil {
		select {
		case out <- scraper.Error(fmt.Errorf("dvd page: %w", err)):
		case <-ctx.Done():
		}
		return
	}

	scenes := parseSceneBlocks(body)
	if len(scenes) == 0 {
		return
	}

	select {
	case out <- scraper.Progress(len(scenes)):
	case <-ctx.Done():
		return
	}

	s.emitScenes(ctx, scenes, studioURL, now, opts, out)
}

func (s *Scraper) emitScenes(ctx context.Context, scenes []parsedScene, studioURL string, now time.Time, opts scraper.ListOpts, out chan<- scraper.SceneResult) (stopped bool) {
	for _, ps := range scenes {
		if len(opts.KnownIDs) > 0 && opts.KnownIDs[ps.id] {
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
	return io.ReadAll(resp.Body)
}

type parsedScene struct {
	id         string
	title      string
	relURL     string
	thumbnail  string
	performers []string
	date       string
	duration   string
}

var (
	sceneStartRe = regexp.MustCompile(`<div\s+class="latestUpdateB"\s+data-setid="(\d+)">`)
	titleRe      = regexp.MustCompile(`(?s)<h4\s+class="link_bright">\s*<a\s+[^>]*href="([^"]*)"[^>]*>([^<]+)</a>`)
	thumbRe      = regexp.MustCompile(`src0_4x="([^"]+)"`)
	thumbFallRe  = regexp.MustCompile(`src0_1x="([^"]+)"`)
	performerRe  = regexp.MustCompile(`<a\s+class="[^"]*infolink[^"]*"\s+href="[^"]*">([^<]+)</a>`)
	dateRe       = regexp.MustCompile(`<!-- Date -->\s*(\d{2}/\d{2}/\d{4})`)
	durationRe   = regexp.MustCompile(`fa-video"></i>(\d+)\s*min`)
	paginationRe = regexp.MustCompile(`(?s)<div\s+class="pagination[^"]*">(.*?)</div>`)
	maxPageRe    = regexp.MustCompile(`(?:movies_|page=)(\d+)`)
	modelIDRe    = regexp.MustCompile(`sets\.php\?id=(\d+)`)
)

func parseSceneBlocks(body []byte) []parsedScene {
	locs := sceneStartRe.FindAllSubmatchIndex(body, -1)
	scenes := make([]parsedScene, 0, len(locs))

	for i, loc := range locs {
		end := len(body)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		block := body[loc[0]:end]

		ps := parsedScene{
			id: string(body[loc[2]:loc[3]]),
		}

		if tm := titleRe.FindSubmatch(block); tm != nil {
			ps.relURL = string(tm[1])
			ps.title = html.UnescapeString(strings.TrimSpace(string(tm[2])))
		}

		if tm := thumbRe.FindSubmatch(block); tm != nil {
			ps.thumbnail = string(tm[1])
		} else if tm := thumbFallRe.FindSubmatch(block); tm != nil {
			ps.thumbnail = string(tm[1])
		}

		for _, pm := range performerRe.FindAllSubmatch(block, -1) {
			name := strings.TrimSpace(string(pm[1]))
			if name != "" {
				ps.performers = append(ps.performers, name)
			}
		}

		if dm := dateRe.FindSubmatch(block); dm != nil {
			ps.date = string(dm[1])
		}

		if dm := durationRe.FindSubmatch(block); dm != nil {
			ps.duration = string(dm[1])
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
	for _, m := range maxPageRe.FindAllSubmatch(pm[1], -1) {
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
		ID:         ps.id,
		SiteID:     "sofiemarie",
		StudioURL:  studioURL,
		Title:      ps.title,
		URL:        sceneURL,
		Thumbnail:  thumb,
		Performers: ps.performers,
		Date:       parseDate(ps.date),
		Duration:   parseDuration(ps.duration),
		Studio:     "Sofie Marie",
		ScrapedAt:  now,
	}
	scene.AddPrice(models.PriceSnapshot{Date: now, IsFree: false})
	return scene
}

func parseDate(s string) time.Time {
	t, err := time.Parse("01/02/2006", s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

func parseDuration(s string) int {
	if s == "" {
		return 0
	}
	n, _ := strconv.Atoi(s)
	return n * 60
}
