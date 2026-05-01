package grandparentsx

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
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

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return "grandparentsx" }

func (s *Scraper) Patterns() []string {
	return []string{
		"grandparentsx.com",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?grandparentsx\.com/?$`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// ---- runner ----

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	body, err := s.fetchPage(ctx, studioURL)
	if err != nil {
		select {
		case out <- scraper.Error(fmt.Errorf("homepage: %w", err)):
		case <-ctx.Done():
		}
		return
	}

	items := parseScenes(body)
	if len(items) == 0 {
		return
	}

	select {
	case out <- scraper.Progress(len(items)):
	case <-ctx.Done():
		return
	}

	for _, item := range items {
		if opts.KnownIDs[item.id] {
			select {
			case out <- scraper.StoppedEarly():
			case <-ctx.Done():
			}
			return
		}
		scene := item.toScene(studioURL)
		select {
		case out <- scraper.Scene(scene):
		case <-ctx.Done():
			return
		}
	}
}

// ---- parsing ----

type sceneItem struct {
	id       string
	title    string
	thumb    string
	date     string
	duration int
	sceneURL string
}

var (
	blockRe    = regexp.MustCompile(`(?s)<div class='video-wrapper[^']*'>(.*?)</div>\s*</div>\s*</div>`)
	galleryRe  = regexp.MustCompile(`galleryId=(\d+)`)
	titleRe    = regexp.MustCompile(`<span class="featuring">([^<]+)</span>`)
	thumbRe    = regexp.MustCompile(`data-image="([^"]+)"`)
	dateRe     = regexp.MustCompile(`<div class="featuring2">([^<]+)</div>`)
	durationRe = regexp.MustCompile(`fa-clock-o"></i>\s*(\d+)\s*min`)
)

func parseScenes(body []byte) []sceneItem {
	blocks := blockRe.FindAllSubmatch(body, -1)
	seen := map[string]bool{}
	var items []sceneItem

	for _, b := range blocks {
		block := b[1]

		gm := galleryRe.FindSubmatch(block)
		if gm == nil {
			continue
		}
		id := string(gm[1])
		if seen[id] {
			continue
		}
		seen[id] = true

		item := sceneItem{
			id:       id,
			sceneURL: "https://grandparentsx.com/#" + id,
		}

		if m := titleRe.FindSubmatch(block); m != nil {
			item.title = strings.TrimSpace(string(m[1]))
		}

		if m := thumbRe.FindSubmatch(block); m != nil {
			item.thumb = string(m[1])
		}

		if m := dateRe.FindSubmatch(block); m != nil {
			item.date = strings.TrimSpace(string(m[1]))
		}

		if m := durationRe.FindSubmatch(block); m != nil {
			mins, _ := strconv.Atoi(string(m[1]))
			item.duration = mins * 60
		}

		items = append(items, item)
	}
	return items
}

func (item sceneItem) toScene(studioURL string) models.Scene {
	scene := models.Scene{
		ID:        item.id,
		SiteID:    "grandparentsx",
		StudioURL: studioURL,
		URL:       item.sceneURL,
		Title:     item.title,
		Thumbnail: item.thumb,
		Duration:  item.duration,
		Studio:    "GrandparentsX",
		ScrapedAt: time.Now().UTC(),
	}

	if item.date != "" {
		if t, err := time.Parse("January 02, 2006", item.date); err == nil {
			scene.Date = t
		}
	}

	return scene
}

// ---- HTTP ----

func (s *Scraper) fetchPage(ctx context.Context, rawURL string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: rawURL,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentChrome,
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}
