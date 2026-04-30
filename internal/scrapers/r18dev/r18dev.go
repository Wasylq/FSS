package r18dev

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
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

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return "r18dev" }

func (s *Scraper) Patterns() []string {
	return []string{
		"r18.dev/videos/vod/movies/list/?id={id}&type=actress",
		"r18.dev/videos/vod/movies/list/?id={id}&type=studio",
		"r18.dev/videos/vod/movies/list/?id={id}&type=category",
		"r18.dev/videos/vod/movies/list/?id={id}&type=director",
	}
}

var matchRe = regexp.MustCompile(
	`^https?://(?:www\.)?r18\.dev/videos/vod/movies/list/?(?:\?|$)`,
)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// ---- JSON API types ----

type listResponse struct {
	Results      []listItem `json:"results"`
	TotalResults int        `json:"total_results"`
}

type listItem struct {
	ContentID string  `json:"content_id"`
	DvdID     *string `json:"dvd_id"`
}

type detailResponse struct {
	ContentID      string     `json:"content_id"`
	DvdID          *string    `json:"dvd_id"`
	TitleJA        *string    `json:"title_ja"`
	TitleEN        *string    `json:"title_en"`
	CommentEN      *string    `json:"comment_en"`
	ReleaseDate    *string    `json:"release_date"`
	RuntimeMins    *int       `json:"runtime_mins"`
	JacketFullURL  *string    `json:"jacket_full_url"`
	JacketThumbURL *string    `json:"jacket_thumb_url"`
	MakerNameEN    *string    `json:"maker_name_en"`
	MakerNameJA    *string    `json:"maker_name_ja"`
	LabelNameEN    *string    `json:"label_name_en"`
	SeriesID       *int       `json:"series_id"`
	Actresses      []person   `json:"actresses"`
	Directors      []person   `json:"directors"`
	Categories     []category `json:"categories"`
}

type person struct {
	NameKanji  string `json:"name_kanji"`
	NameRomaji string `json:"name_romaji"`
}

type category struct {
	NameEN string `json:"name_en"`
	NameJA string `json:"name_ja"`
}

// ---- runner ----

const defaultDelay = 500 * time.Millisecond

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	delay := opts.Delay
	if delay == 0 {
		delay = defaultDelay
	}

	workers := opts.Workers
	if workers <= 0 {
		workers = 4
	}

	work := make(chan listItem)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range work {
				scene, err := s.fetchDetail(ctx, studioURL, item, delay)
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

	go func() {
		defer close(work)

		seen := map[string]bool{}

		for page := 1; ; page++ {
			if page > 1 {
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return
				}
			}

			apiURL := buildListURL(studioURL, page)
			var lr listResponse
			if err := s.fetchJSON(ctx, apiURL, &lr); err != nil {
				select {
				case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
				case <-ctx.Done():
				}
				return
			}

			if len(lr.Results) == 0 {
				return
			}

			if page == 1 {
				total := lr.TotalResults
				if total <= 0 {
					total = len(lr.Results)
				}
				select {
				case out <- scraper.Progress(total):
				case <-ctx.Done():
					return
				}
			}

			for _, item := range lr.Results {
				id := itemID(item)
				if opts.KnownIDs[id] {
					select {
					case out <- scraper.StoppedEarly():
					case <-ctx.Done():
					}
					return
				}
				if seen[id] {
					continue
				}
				seen[id] = true
				select {
				case work <- item:
				case <-ctx.Done():
					return
				}
			}

			if page*100 >= lr.TotalResults {
				return
			}
		}
	}()

	wg.Wait()
}

// ---- URL helpers ----

func buildListURL(studioURL string, page int) string {
	u, err := url.Parse(studioURL)
	if err != nil {
		return studioURL
	}
	q := u.Query()
	q.Set("page", fmt.Sprintf("%d", page))
	return fmt.Sprintf("%s://%s/videos/vod/movies/list2/json?%s", u.Scheme, u.Host, q.Encode())
}

func buildDetailURL(studioURL, contentID string) string {
	u, err := url.Parse(studioURL)
	if err != nil {
		return fmt.Sprintf("https://r18.dev/videos/vod/movies/detail/-/combined=%s/json", contentID)
	}
	return fmt.Sprintf("%s://%s/videos/vod/movies/detail/-/combined=%s/json", u.Scheme, u.Host, contentID)
}

func sceneURL(contentID string) string {
	return fmt.Sprintf("https://r18.dev/videos/vod/movies/detail/-/id=%s/", contentID)
}

func itemID(item listItem) string {
	if item.DvdID != nil && *item.DvdID != "" {
		return *item.DvdID
	}
	return strings.ToUpper(item.ContentID)
}

// ---- detail fetching ----

func (s *Scraper) fetchDetail(ctx context.Context, studioURL string, item listItem, delay time.Duration) (models.Scene, error) {
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return models.Scene{}, ctx.Err()
		}
	}

	var dr detailResponse
	if err := s.fetchJSON(ctx, buildDetailURL(studioURL, item.ContentID), &dr); err != nil {
		return models.Scene{}, fmt.Errorf("detail %s: %w", item.ContentID, err)
	}

	return toScene(studioURL, dr), nil
}

func toScene(studioURL string, dr detailResponse) models.Scene {
	id := strings.ToUpper(dr.ContentID)
	if dr.DvdID != nil && *dr.DvdID != "" {
		id = *dr.DvdID
	}

	studio := "r18.dev"
	if dr.MakerNameEN != nil && *dr.MakerNameEN != "" {
		studio = *dr.MakerNameEN
	} else if dr.MakerNameJA != nil && *dr.MakerNameJA != "" {
		studio = *dr.MakerNameJA
	}

	scene := models.Scene{
		ID:        id,
		SiteID:    "r18dev",
		StudioURL: studioURL,
		URL:       sceneURL(dr.ContentID),
		Studio:    studio,
		ScrapedAt: time.Now().UTC(),
	}

	if dr.TitleJA != nil && *dr.TitleJA != "" {
		scene.Title = *dr.TitleJA
	} else if dr.TitleEN != nil {
		scene.Title = *dr.TitleEN
	}

	if dr.CommentEN != nil && *dr.CommentEN != "" {
		scene.Description = *dr.CommentEN
	}

	if dr.JacketFullURL != nil && *dr.JacketFullURL != "" {
		scene.Thumbnail = *dr.JacketFullURL
	} else if dr.JacketThumbURL != nil && *dr.JacketThumbURL != "" {
		scene.Thumbnail = *dr.JacketThumbURL
	}

	if dr.ReleaseDate != nil && *dr.ReleaseDate != "" {
		if t, err := time.Parse("2006-01-02", *dr.ReleaseDate); err == nil {
			scene.Date = t
		}
	}

	if dr.RuntimeMins != nil && *dr.RuntimeMins > 0 {
		scene.Duration = *dr.RuntimeMins * 60
	}

	for _, a := range dr.Actresses {
		name := a.NameRomaji
		if name == "" {
			name = a.NameKanji
		}
		if name != "" {
			scene.Performers = append(scene.Performers, name)
		}
	}

	for _, d := range dr.Directors {
		name := d.NameRomaji
		if name == "" {
			name = d.NameKanji
		}
		if name != "" {
			scene.Director = name
			break
		}
	}

	for _, c := range dr.Categories {
		name := c.NameEN
		if name == "" {
			name = c.NameJA
		}
		if name != "" {
			scene.Tags = append(scene.Tags, name)
		}
	}

	return scene
}

// ---- HTTP ----

func (s *Scraper) fetchJSON(ctx context.Context, rawURL string, v any) error {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: rawURL,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentChrome,
			"Accept":     "application/json",
		},
	})
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return json.NewDecoder(resp.Body).Decode(v)
}
