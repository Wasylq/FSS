package puretaboo

import (
	"context"
	"encoding/json"
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

const (
	algoliaAppID = "TSMKFA364Q"
	algoliaHost  = "https://TSMKFA364Q-dsn.algolia.net"
	indexName    = "all_scenes_latest_desc"
	imageCDN     = "https://transform.gammacdn.com/movies"
	siteBase     = "https://www.puretaboo.com"
	hitsPerPage  = 100
)

type Scraper struct {
	client      *http.Client
	siteBase    string
	algoliaHost string
}

func New() *Scraper {
	return &Scraper{
		client:      httpx.NewClient(30 * time.Second),
		siteBase:    siteBase,
		algoliaHost: algoliaHost,
	}
}

func init() {
	scraper.Register(New())
}

func (s *Scraper) ID() string { return "puretaboo" }

func (s *Scraper) Patterns() []string {
	return []string{"puretaboo.com"}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?puretaboo\.com`)

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var apiKeyRe = regexp.MustCompile(`"algolia"\s*:\s*\{[^}]*"apiKey"\s*:\s*"([^"]+)"`)

func (s *Scraper) fetchAPIKey(ctx context.Context) (string, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: s.siteBase + "/en/videos",
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentFirefox,
			"Accept":     "text/html",
		},
	})
	if err != nil {
		return "", fmt.Errorf("fetching API key: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading page for API key: %w", err)
	}

	m := apiKeyRe.FindSubmatch(body)
	if m == nil {
		return "", fmt.Errorf("algolia API key not found in page source")
	}
	return string(m[1]), nil
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	apiKey, err := s.fetchAPIKey(ctx)
	if err != nil {
		select {
		case out <- scraper.SceneResult{Err: err}:
		case <-ctx.Done():
		}
		return
	}

	for page := 0; ; page++ {
		if ctx.Err() != nil {
			return
		}

		if page > 0 && opts.Delay > 0 {
			select {
			case <-time.After(opts.Delay):
			case <-ctx.Done():
				return
			}
		}

		hits, total, err := s.fetchPage(ctx, apiKey, page)
		if err != nil {
			select {
			case out <- scraper.SceneResult{Err: fmt.Errorf("page %d: %w", page, err)}:
			case <-ctx.Done():
			}
			return
		}

		if len(hits) == 0 {
			return
		}

		if page == 0 && total > 0 {
			select {
			case out <- scraper.SceneResult{Total: total}:
			case <-ctx.Done():
				return
			}
		}

		now := time.Now().UTC()
		for _, hit := range hits {
			id := strconv.Itoa(hit.ClipID)
			if len(opts.KnownIDs) > 0 && opts.KnownIDs[id] {
				select {
				case out <- scraper.SceneResult{StoppedEarly: true}:
				case <-ctx.Done():
				}
				return
			}

			scene := toScene(studioURL, hit, now)
			select {
			case out <- scraper.SceneResult{Scene: scene}:
			case <-ctx.Done():
				return
			}
		}

		if (page+1)*hitsPerPage >= total {
			return
		}
	}
}

func (s *Scraper) fetchPage(ctx context.Context, apiKey string, page int) ([]algoliaHit, int, error) {
	query := algoliaQuery{
		Query:       "",
		HitsPerPage: hitsPerPage,
		Page:        page,
		Filters:     "availableOnSite:puretaboo AND upcoming:0",
	}
	body, err := json.Marshal(query)
	if err != nil {
		return nil, 0, err
	}

	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:  fmt.Sprintf("%s/1/indexes/%s/query", s.algoliaHost, indexName),
		Body: body,
		Headers: map[string]string{
			"x-algolia-application-id": algoliaAppID,
			"x-algolia-api-key":        apiKey,
			"Referer":                  s.siteBase + "/",
			"Content-Type":            "application/json",
		},
	})
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result algoliaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, 0, fmt.Errorf("decoding algolia response: %w", err)
	}
	return result.Hits, result.NbHits, nil
}

func toScene(studioURL string, hit algoliaHit, now time.Time) models.Scene {
	performers := make([]string, len(hit.Actors))
	for i, a := range hit.Actors {
		performers[i] = a.Name
	}

	tags := make([]string, 0, len(hit.Categories))
	for _, c := range hit.Categories {
		tags = append(tags, c.Name)
	}

	var directors []string
	for _, d := range hit.Directors {
		directors = append(directors, d.Name)
	}
	director := strings.Join(directors, ", ")

	width, height, resolution := bestResolution(hit.VideoFormats, hit.MasterCategories)

	thumbnail := thumbnailURL(hit.Pictures)

	var preview string
	if t := bestTrailer(hit.VideoFormats); t != "" {
		preview = t
	}

	desc := hit.Description
	desc = strings.ReplaceAll(desc, "</br>", "\n")
	desc = strings.ReplaceAll(desc, "<br>", "\n")
	desc = strings.ReplaceAll(desc, "<br/>", "\n")
	desc = strings.ReplaceAll(desc, "<br />", "\n")
	desc = html.UnescapeString(desc)

	sceneURL := fmt.Sprintf("%s/en/video/puretaboo/%s/%d", siteBase, hit.URLTitle, hit.ClipID)

	scene := models.Scene{
		ID:          strconv.Itoa(hit.ClipID),
		SiteID:      "puretaboo",
		StudioURL:   studioURL,
		Title:       hit.Title,
		URL:         sceneURL,
		Date:        parseDate(hit.ReleaseDate),
		Description: desc,
		Thumbnail:   thumbnail,
		Preview:     preview,
		Performers:  performers,
		Director:    director,
		Studio:      "Pure Taboo",
		Tags:        tags,
		Series:      hit.SerieName,
		Duration:    hit.Length,
		Resolution:  resolution,
		Width:       width,
		Height:      height,
		Likes:       hit.RatingsUp,
		ScrapedAt:   now,
	}

	return scene
}

func parseDate(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

func bestResolution(formats []videoFormat, masterCats []string) (width, height int, resolution string) {
	for _, mc := range masterCats {
		if mc == "4k" {
			return 3840, 2160, "2160p"
		}
	}
	best := 0
	for _, f := range formats {
		h := parseFormatHeight(f.Format)
		if h > best {
			best = h
		}
	}
	if best == 0 {
		return 0, 0, ""
	}
	height = best
	resolution = fmt.Sprintf("%dp", best)
	switch best {
	case 2160:
		width = 3840
	case 1080:
		width = 1920
	case 720:
		width = 1280
	case 576:
		width = 1024
	case 432:
		width = 768
	case 288:
		width = 512
	}
	return width, height, resolution
}

func parseFormatHeight(s string) int {
	s = strings.TrimSuffix(s, "p")
	n, _ := strconv.Atoi(s)
	return n
}

func thumbnailURL(pics pictures) string {
	if pics.Full1920 != "" {
		return imageCDN + pics.Full1920
	}
	if pics.NSFW.Top.Full1920 != "" {
		return imageCDN + pics.NSFW.Top.Full1920
	}
	if pics.Res638 != "" {
		return imageCDN + pics.Res638
	}
	return ""
}

func bestTrailer(formats []videoFormat) string {
	best := 0
	var url string
	for _, f := range formats {
		h := parseFormatHeight(f.Format)
		if h > best && f.TrailerURL != "" {
			best = h
			url = f.TrailerURL
		}
	}
	return url
}

// ---- Algolia API types ----

type algoliaQuery struct {
	Query       string `json:"query"`
	HitsPerPage int    `json:"hitsPerPage"`
	Page        int    `json:"page"`
	Filters     string `json:"filters"`
}

type algoliaResponse struct {
	Hits    []algoliaHit `json:"hits"`
	NbHits  int          `json:"nbHits"`
	NbPages int          `json:"nbPages"`
}

type algoliaHit struct {
	ClipID           int           `json:"clip_id"`
	Title            string        `json:"title"`
	Description      string        `json:"description"`
	ClipLength       string        `json:"clip_length"`
	Length           int           `json:"length"`
	ReleaseDate      string        `json:"release_date"`
	SiteName         string        `json:"sitename"`
	SiteNamePretty   string        `json:"sitename_pretty"`
	StudioName       string        `json:"studio_name"`
	SerieName        string        `json:"serie_name"`
	URLTitle         string        `json:"url_title"`
	Actors           []actor       `json:"actors"`
	Directors        []director    `json:"directors"`
	Categories       []category    `json:"categories"`
	VideoFormats     []videoFormat `json:"video_formats"`
	Pictures         pictures      `json:"pictures"`
	MasterCategories []string      `json:"master_categories"`
	RatingsUp        int           `json:"ratings_up"`
	RatingsDown      int           `json:"ratings_down"`
	ObjectID         string        `json:"objectID"`
}

type actor struct {
	ActorID string `json:"actor_id"`
	Name    string `json:"name"`
	Gender  string `json:"gender"`
	URLName string `json:"url_name"`
}

type director struct {
	Name    string `json:"name"`
	URLName string `json:"url_name"`
}

type category struct {
	CategoryID string `json:"category_id"`
	Name       string `json:"name"`
	URLName    string `json:"url_name"`
}

type videoFormat struct {
	Codec      string `json:"codec"`
	Format     string `json:"format"`
	Size       string `json:"size"`
	Slug       string `json:"slug"`
	TrailerURL string `json:"trailer_url"`
}

type pictures struct {
	Full1920 string `json:"1920x1080"`
	Res638   string `json:"638x360"`
	NSFW     struct {
		Top struct {
			Full1920 string `json:"1920x1080"`
		} `json:"top"`
	} `json:"nsfw"`
	SFW struct {
		Top struct {
			Full1920 string `json:"1920x1080"`
		} `json:"top"`
	} `json:"sfw"`
}
