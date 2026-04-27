package ladysonia

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

const defaultSiteBase = "https://tour.lady-sonia.com"

type Scraper struct {
	client   *http.Client
	siteBase string
}

func New() *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second), siteBase: defaultSiteBase}
}

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return "ladysonia" }

func (s *Scraper) Patterns() []string {
	return []string{"lady-sonia.com"}
}

var matchRe = regexp.MustCompile(`^https?://(?:(?:www|tour)\.)?lady-sonia\.com`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, _ string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, s.siteBase, opts, out)
	return out, nil
}

var nextDataRe = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__"[^>]*>(.*?)</script>`)

type nextData struct {
	Props struct {
		PageProps struct {
			Contents pageContents `json:"contents"`
		} `json:"pageProps"`
	} `json:"props"`
}

type pageContents struct {
	Total      int         `json:"total"`
	Page       json.Number `json:"page"`
	TotalPages int         `json:"total_pages"`
	Data       []scene     `json:"data"`
}

type scene struct {
	ID               int                    `json:"id"`
	Title            string                 `json:"title"`
	Slug             string                 `json:"slug"`
	Description      string                 `json:"description"`
	PublishDate      string                 `json:"publish_date"`
	SecondsDuration  int                    `json:"seconds_duration"`
	Models           []string               `json:"models"`
	Tags             []string               `json:"tags"`
	Thumb            string                 `json:"thumb"`
	TrailerScreencap string                 `json:"trailer_screencap"`
	Site             string                 `json:"site"`
	Videos           map[string]videoFormat `json:"videos"`
}

type videoFormat struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

func (s *Scraper) run(ctx context.Context, base string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	for page := 1; ; page++ {
		url := fmt.Sprintf("%s/scenes?page=%d", base, page)
		body, err := s.fetchPage(ctx, url)
		if err != nil {
			select {
			case out <- scraper.SceneResult{Err: fmt.Errorf("page %d: %w", page, err)}:
			case <-ctx.Done():
			}
			return
		}

		contents, err := parsePage(body)
		if err != nil {
			select {
			case out <- scraper.SceneResult{Err: fmt.Errorf("page %d: %w", page, err)}:
			case <-ctx.Done():
			}
			return
		}

		if page == 1 {
			select {
			case out <- scraper.SceneResult{Total: contents.Total}:
			case <-ctx.Done():
				return
			}
		}

		for _, sc := range contents.Data {
			id := strconv.Itoa(sc.ID)
			if opts.KnownIDs[id] {
				select {
				case out <- scraper.SceneResult{StoppedEarly: true}:
				case <-ctx.Done():
				}
				return
			}
			select {
			case out <- scraper.SceneResult{Scene: toScene(base, sc)}:
			case <-ctx.Done():
				return
			}
		}

		if page >= contents.TotalPages {
			return
		}

		select {
		case <-time.After(opts.Delay):
		case <-ctx.Done():
			return
		}
	}
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

func parsePage(body []byte) (*pageContents, error) {
	m := nextDataRe.FindSubmatch(body)
	if m == nil {
		return nil, fmt.Errorf("__NEXT_DATA__ not found")
	}
	var nd nextData
	if err := json.Unmarshal(m[1], &nd); err != nil {
		return nil, fmt.Errorf("parse __NEXT_DATA__: %w", err)
	}
	return &nd.Props.PageProps.Contents, nil
}

func toScene(base string, sc scene) models.Scene {
	id := strconv.Itoa(sc.ID)
	sceneURL := fmt.Sprintf("%s/scenes/%s", base, sc.Slug)

	var date time.Time
	if t, err := time.Parse("2006/01/02 15:04:05", sc.PublishDate); err == nil {
		date = t.UTC()
	}

	desc := html.UnescapeString(sc.Description)
	desc = strings.ReplaceAll(desc, " ", " ")
	desc = strings.TrimSpace(desc)

	thumb := sc.TrailerScreencap
	if thumb == "" {
		thumb = sc.Thumb
	}

	var width, height int
	for _, key := range []string{"orig", "hq", "stream", "mobile"} {
		if vf, ok := sc.Videos[key]; ok && vf.Width > width {
			width = vf.Width
			height = vf.Height
		}
	}

	var resolution string
	if height >= 2160 {
		resolution = "2160p"
	} else if height >= 1080 {
		resolution = "1080p"
	} else if height >= 720 {
		resolution = "720p"
	} else if height >= 480 {
		resolution = "480p"
	}

	return models.Scene{
		ID:          id,
		SiteID:      "ladysonia",
		StudioURL:   base,
		Title:       sc.Title,
		URL:         sceneURL,
		Date:        date,
		Description: desc,
		Duration:    sc.SecondsDuration,
		Performers:  sc.Models,
		Tags:        sc.Tags,
		Thumbnail:   thumb,
		Studio:      sc.Site,
		Width:       width,
		Height:      height,
		Resolution:  resolution,
		ScrapedAt:   time.Now().UTC(),
	}
}
