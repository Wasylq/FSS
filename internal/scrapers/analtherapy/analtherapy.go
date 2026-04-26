package analtherapy

import (
	"context"
	"net/http"
	"regexp"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/internal/scrapers/wputil"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

type Scraper struct {
	client   *http.Client
	siteBase string
	headers  map[string]string
}

func New() *Scraper {
	return &Scraper{
		client:   httpx.NewClient(30 * time.Second),
		siteBase: "https://analtherapyxxx.com",
		headers:  wputil.BrowserHeaders(),
	}
}

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return "analtherapy" }

func (s *Scraper) Patterns() []string {
	return []string{"analtherapyxxx.com"}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?analtherapyxxx\.com`)

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	if opts.Workers <= 0 {
		opts.Workers = 3
	}
	out := make(chan scraper.SceneResult)
	go func() {
		defer close(out)
		wputil.RunWorkerPool(ctx, s.client, s.headers,
			[]string{s.siteBase + "/sitemap.xml"},
			studioURL, opts, parsePage, out)
	}()
	return out, nil
}

var titleSuffixRe = regexp.MustCompile(`\s*\|\s*Anal Therapy XXX$`)

func parsePage(studioURL, pageURL string, body []byte, now time.Time) (models.Scene, bool, error) {
	meta := wputil.ParseMeta(body, "")
	meta.Title = titleSuffixRe.ReplaceAllString(meta.Title, "")

	if !meta.HasVideo {
		return models.Scene{}, true, nil
	}

	id := meta.PostID
	if id == "" {
		id = wputil.SlugFromURL(pageURL)
	}

	scene := models.Scene{
		ID:          id,
		SiteID:      "analtherapy",
		StudioURL:   studioURL,
		Title:       meta.Title,
		URL:         pageURL,
		Date:        meta.Date,
		Description: meta.Description,
		Thumbnail:   meta.Thumbnail,
		Studio:      "Anal Therapy",
		ScrapedAt:   now,
	}

	return scene, false, nil
}
