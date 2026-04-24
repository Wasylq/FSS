package momcomesfirst

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
		siteBase: "https://momcomesfirst.com",
		headers:  wputil.BrowserHeaders(),
	}
}

func init() {
	scraper.Register(New())
}

func (s *Scraper) ID() string { return "momcomesfirst" }

func (s *Scraper) Patterns() []string {
	return []string{"momcomesfirst.com"}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?momcomesfirst\.com`)

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

// titleSuffixRe strips the tag list and site name from the og:title.
// Format: "Title - tag1, tag2, performer - Mom Comes First"
var titleSuffixRe = regexp.MustCompile(`\s*-\s*(?:[^-]+-\s*)?Mom Comes First$`)

func parsePage(studioURL, pageURL string, body []byte, now time.Time) (models.Scene, bool, error) {
	meta := wputil.ParseMeta(body, "")

	// Strip the compound suffix: " - tags - Mom Comes First"
	if meta.Title != "" {
		meta.Title = titleSuffixRe.ReplaceAllString(meta.Title, "")
	}

	// Skip non-video pages (homepage, about, etc.)
	if meta.PostID == "" && len(meta.Tags) == 0 {
		return models.Scene{}, true, nil
	}

	id := meta.PostID
	if id == "" {
		id = wputil.SlugFromURL(pageURL)
	}

	// Combine article:tag and articleSection categories into tags.
	tagSet := make(map[string]bool)
	var tags []string
	for _, t := range meta.Tags {
		if !tagSet[t] {
			tagSet[t] = true
			tags = append(tags, t)
		}
	}
	for _, c := range meta.Categories {
		if !tagSet[c] {
			tagSet[c] = true
			tags = append(tags, c)
		}
	}

	width := meta.Width
	height := meta.Height
	resolution := ""
	if width == 0 && height > 0 {
		width = wputil.VideoWidth(height)
	}
	if height >= 2160 {
		resolution = "4K"
	} else if height >= 1080 {
		resolution = "1080p"
	} else if height >= 720 {
		resolution = "720p"
	}

	scene := models.Scene{
		ID:          id,
		SiteID:      "momcomesfirst",
		StudioURL:   studioURL,
		Title:       meta.Title,
		URL:         pageURL,
		Date:        meta.Date,
		Description: meta.Description,
		Thumbnail:   meta.Thumbnail,
		Studio:      "Mom Comes First",
		Tags:        tags,
		Width:       width,
		Height:      height,
		Resolution:  resolution,
		ScrapedAt:   now,
	}

	return scene, false, nil
}
