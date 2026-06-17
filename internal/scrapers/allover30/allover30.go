package allover30

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
	"github.com/Wasylq/FSS/parseutil"
	"github.com/Wasylq/FSS/scraper"
)

type Scraper struct {
	client *http.Client
}

func New() *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second)}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return "allover30" }

func (s *Scraper) Patterns() []string {
	return []string{
		"new.allover30.com/model-pages/{slug}/{id}",
	}
}

var (
	matchRe      = regexp.MustCompile(`^https?://(?:new\.)?allover30\.com`)
	modelPageRe  = regexp.MustCompile(`/model-pages/[^/]+/\d+`)
	modelNameRe  = regexp.MustCompile(`(?s)<div class="modelInfo">.*?<h3>([^<]+)</h3>`)
	movieBlockRe = regexp.MustCompile(`(?s)<div class="modelBox\s+vid">(.*?)</ul>`)
	thumbRe      = regexp.MustCompile(`src="(https://static\.allover30\.com/\w/[^/]+/(\d+)/cover\.jpg)"`)
	dateRe       = regexp.MustCompile(`<strong>Date added:</strong>\s*([^<]+)`)
	categoryRe   = regexp.MustCompile(`<strong>Category:</strong>\s*<a[^>]*>([^<]+)</a>`)
)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	if !modelPageRe.MatchString(studioURL) {
		return nil, fmt.Errorf("allover30: studio-level scraping is not supported — provide a model page URL like https://new.allover30.com/model-pages/ryan-keely/1549")
	}
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	scraper.Debugf(1, "allover30: scraping model page")
	body, err := s.fetchPage(ctx, studioURL)
	if err != nil {
		select {
		case out <- scraper.Error(err):
		case <-ctx.Done():
		}
		return
	}

	modelName := "Unknown"
	if m := modelNameRe.FindSubmatch(body); m != nil {
		modelName = strings.TrimSpace(html.UnescapeString(string(m[1])))
	}
	scraper.Debugf(1, "allover30: model %s", modelName)

	movies := movieBlockRe.FindAll(body, -1)
	scraper.Debugf(1, "allover30: %d movies found", len(movies))

	if len(movies) == 0 {
		return
	}

	select {
	case out <- scraper.Progress(len(movies)):
	case <-ctx.Done():
		return
	}

	now := time.Now().UTC()
	for _, block := range movies {
		if ctx.Err() != nil {
			return
		}

		scene := parseMovie(block, modelName, studioURL, now)
		if scene.ID == "" {
			continue
		}

		if opts.KnownIDs[scene.ID] {
			select {
			case out <- scraper.StoppedEarly():
			case <-ctx.Done():
			}
			return
		}

		select {
		case out <- scraper.Scene(scene):
		case <-ctx.Done():
			return
		}
	}
}

func parseMovie(block []byte, modelName, studioURL string, now time.Time) models.Scene {
	var scene models.Scene
	scene.SiteID = "allover30"
	scene.StudioURL = studioURL
	scene.Studio = "AllOver30"
	scene.Performers = []string{modelName}
	scene.ScrapedAt = now

	if m := thumbRe.FindSubmatch(block); m != nil {
		scene.Thumbnail = string(m[1])
		scene.ID = string(m[2])
	}

	var category string
	if m := categoryRe.FindSubmatch(block); m != nil {
		category = strings.TrimSpace(html.UnescapeString(string(m[1])))
		scene.Tags = []string{category}
	}

	if category != "" {
		scene.Title = fmt.Sprintf("%s — %s", modelName, category)
	} else {
		scene.Title = modelName
	}

	scene.URL = studioURL

	if m := dateRe.FindSubmatch(block); m != nil {
		raw := strings.TrimSpace(string(m[1]))
		if t, err := parseutil.TryParseDate(parseutil.StripOrdinalSuffix(raw), "Jan 2, 2006"); err == nil {
			scene.Date = t
		}
	}

	return scene
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
