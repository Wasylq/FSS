// Package hustler scrapes Hustler Unlimited (hustlerunlimited.com). The site
// runs WordPress with an open REST API and a custom `videos` post type plus
// custom taxonomies (hu_actors, video_tags, video_channels, video_studio). The
// scenes are the `videos` objects, fetched with _embed so taxonomy term names
// resolve inline.
package hustler

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const siteBase = "https://hustlerunlimited.com"

type Scraper struct {
	client *http.Client
}

func New() *Scraper { return &Scraper{client: httpx.NewClient(30 * time.Second)} }

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return "hustler" }
func (s *Scraper) Patterns() []string {
	return []string{"hustlerunlimited.com", "hustlerunlimited.com/videos/{slug}"}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?hustlerunlimited\.com`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

type video struct {
	ID       int        `json:"id"`
	Date     string     `json:"date"`
	Link     string     `json:"link"`
	Title    wpRendered `json:"title"`
	Embedded struct {
		Terms [][]term `json:"wp:term"`
	} `json:"_embedded"`
}

type wpRendered struct {
	Rendered string `json:"rendered"`
}

type term struct {
	Name     string `json:"name"`
	Taxonomy string `json:"taxonomy"`
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, "hustler", out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		vids, total, err := s.fetchPage(ctx, page)
		if err != nil {
			return scraper.PageResult{}, err
		}
		scenes := make([]models.Scene, len(vids))
		for i, v := range vids {
			scenes[i] = toScene(studioURL, v, now)
		}
		return scraper.PageResult{Scenes: scenes, Total: total, Done: len(vids) < 25}, nil
	})
}

func (s *Scraper) fetchPage(ctx context.Context, page int) ([]video, int, error) {
	u := fmt.Sprintf("%s/wp-json/wp/v2/videos?per_page=25&page=%d&orderby=date&order=desc&_embed=1", siteBase, page)
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     u,
		Headers: httpx.BrowserHeaders(httpx.UserAgentChrome),
	})
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	total, _ := strconv.Atoi(resp.Header.Get("X-WP-Total"))
	var vids []video
	if err := httpx.DecodeJSON(resp.Body, &vids); err != nil {
		return nil, 0, fmt.Errorf("decode: %w", err)
	}
	return vids, total, nil
}

func toScene(studioURL string, v video, now time.Time) models.Scene {
	var date time.Time
	if v.Date != "" {
		if t, err := time.Parse("2006-01-02T15:04:05", v.Date); err == nil {
			date = t.UTC()
		}
	}

	scene := models.Scene{
		ID:        strconv.Itoa(v.ID),
		SiteID:    "hustler",
		StudioURL: studioURL,
		Title:     html.UnescapeString(v.Title.Rendered),
		URL:       v.Link,
		Date:      date,
		Studio:    "Hustler",
		ScrapedAt: now,
	}

	for _, grp := range v.Embedded.Terms {
		for _, t := range grp {
			name := html.UnescapeString(t.Name)
			switch t.Taxonomy {
			case "hu_actors":
				scene.Performers = append(scene.Performers, name)
			case "video_tags":
				scene.Tags = append(scene.Tags, name)
			case "video_channels":
				if scene.Series == "" {
					scene.Series = name
				}
				scene.Categories = append(scene.Categories, name)
			case "video_studio":
				if scene.Studio == "Hustler" {
					scene.Studio = name
				}
			case "video_director":
				if scene.Director == "" {
					scene.Director = name
				}
			}
		}
	}
	return scene
}
