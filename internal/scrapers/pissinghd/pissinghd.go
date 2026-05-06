package pissinghd

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"regexp"
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

func (s *Scraper) ID() string { return "pissinghd" }

func (s *Scraper) Patterns() []string {
	return []string{"pissinghd.com"}
}

var matchRe = regexp.MustCompile(`^https?://(?:(?:www|tour)\.)?pissinghd\.com`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

const tourBase = "https://tour.pissinghd.com"

var (
	cardRe   = regexp.MustCompile(`(?s)<div class="col-md-4 col-xs-12 col-sm-6"><!-- Thumbs -->.*?<!-- End Thumbs -->\s*</div>`)
	idRe     = regexp.MustCompile(`data-id="(\d+)"`)
	thumbRe  = regexp.MustCompile(`<img src="(https?://[^"]+)" class="img-responsive thumb"`)
	titleRe  = regexp.MustCompile(`(?s)<div class="tit-title[^"]*">\s*<div[^>]*>([^<]+)</div>`)
	descRe   = regexp.MustCompile(`(?s)<div id="episodedesc\d+"[^>]*>\s*<p[^>]*>(.*?)</p>`)
	tagClean = regexp.MustCompile(`<[^>]+>`)
)

type sceneItem struct {
	id          string
	title       string
	thumbnail   string
	description string
}

func parseListingPage(body []byte) []sceneItem {
	cards := cardRe.FindAll(body, -1)
	items := make([]sceneItem, 0, len(cards))
	for _, card := range cards {
		block := string(card)

		var item sceneItem

		if m := idRe.FindStringSubmatch(block); m != nil {
			item.id = m[1]
		}
		if item.id == "" {
			continue
		}

		if m := titleRe.FindStringSubmatch(block); m != nil {
			item.title = strings.TrimSpace(html.UnescapeString(m[1]))
		}

		if m := thumbRe.FindStringSubmatch(block); m != nil {
			item.thumbnail = m[1]
		}

		if m := descRe.FindStringSubmatch(block); m != nil {
			desc := tagClean.ReplaceAllString(m[1], "")
			item.description = strings.TrimSpace(html.UnescapeString(desc))
		}

		items = append(items, item)
	}
	return items
}

var baseURLRe = regexp.MustCompile(`^(https?://[^/]+)`)

func resolveBase(studioURL string) string {
	if m := baseURLRe.FindString(studioURL); m != "" && !strings.Contains(m, "pissinghd.com") {
		return m
	}
	return tourBase
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	base := resolveBase(studioURL)
	now := time.Now().UTC()
	seen := make(map[string]bool)
	sentTotal := false

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

		pageURL := fmt.Sprintf("%s/videos?page=%d", base, page)
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

		if !sentTotal {
			sentTotal = true
			select {
			case out <- scraper.Progress(len(items)):
			case <-ctx.Done():
				return
			}
		}

		for _, item := range items {
			if seen[item.id] {
				continue
			}
			seen[item.id] = true

			if len(opts.KnownIDs) > 0 && opts.KnownIDs[item.id] {
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
				return
			}

			scene := models.Scene{
				ID:          item.id,
				SiteID:      "pissinghd",
				StudioURL:   studioURL,
				Title:       item.title,
				Thumbnail:   item.thumbnail,
				Description: item.description,
				URL:         fmt.Sprintf("%s/videos?page=%d", tourBase, page),
				Studio:      "Pissing",
				ScrapedAt:   now,
			}

			select {
			case out <- scraper.Scene(scene):
			case <-ctx.Done():
				return
			}
		}
	}
}

func (s *Scraper) fetchPage(ctx context.Context, url string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: url,
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
