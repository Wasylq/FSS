package mylf

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
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
	esEndpoint = "https://tours-store.psmcdn.net/mylf_bundle/_search"
	siteBase   = "https://www.mylf.com"
	pageSize   = 30
)

type Scraper struct {
	client *http.Client
}

func New() *Scraper {
	return &Scraper{
		client: httpx.NewClient(30 * time.Second),
	}
}

func init() {
	scraper.Register(New())
}

func (s *Scraper) ID() string { return "mylf" }

func (s *Scraper) Patterns() []string {
	return []string{
		"mylf.com",
		"mylf.com/models/{slug}",
		"mylf.com/series/{slug}",
		"mylf.com/categories/{name}",
	}
}

var (
	matchRe     = regexp.MustCompile(`^https?://(?:www\.)?mylf\.com`)
	stripTagsRe = regexp.MustCompile(`<[^>]+>`)
)

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

type filterKind int

const (
	filterAll filterKind = iota
	filterModel
	filterSeries
	filterCategory
)

var (
	modelURLRe    = regexp.MustCompile(`/models/([^/?#]+)`)
	seriesURLRe   = regexp.MustCompile(`/series/([^/?#]+)`)
	categoryURLRe = regexp.MustCompile(`/categories/([^/?#]+)`)
)

func classifyURL(u string) (filterKind, string) {
	if m := modelURLRe.FindStringSubmatch(u); m != nil {
		return filterModel, m[1]
	}
	if m := seriesURLRe.FindStringSubmatch(u); m != nil {
		return filterSeries, m[1]
	}
	if m := categoryURLRe.FindStringSubmatch(u); m != nil {
		return filterCategory, m[1]
	}
	return filterAll, ""
}

func buildQuery(kind filterKind, value string) map[string]any {
	must := []any{
		map[string]any{"term": map[string]any{"_doc_type.keyword": "tour_movie"}},
		map[string]any{"term": map[string]any{"isUpcoming": false}},
	}

	switch kind {
	case filterModel:
		must = append(must, map[string]any{"term": map[string]any{"models.id.keyword": value}})
	case filterSeries:
		must = append(must, map[string]any{"term": map[string]any{"site.nickName.keyword": value}})
	case filterCategory:
		tag := strings.ReplaceAll(value, "-", " ")
		tag = titleCase(tag)
		must = append(must, map[string]any{"term": map[string]any{"tags.keyword": tag}})
	}

	return map[string]any{
		"query": map[string]any{
			"bool": map[string]any{"must": must},
		},
		"sort": []any{
			map[string]any{"publishedDate": map[string]any{"order": "desc"}},
		},
	}
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	kind, value := classifyURL(studioURL)
	baseQuery := buildQuery(kind, value)
	now := time.Now().UTC()

	for from := 0; ; from += pageSize {
		if ctx.Err() != nil {
			return
		}
		if from > 0 && opts.Delay > 0 {
			select {
			case <-time.After(opts.Delay):
			case <-ctx.Done():
				return
			}
		}

		baseQuery["from"] = from
		baseQuery["size"] = pageSize

		result, err := s.search(ctx, baseQuery)
		if err != nil {
			select {
			case out <- scraper.SceneResult{Err: fmt.Errorf("from=%d: %w", from, err)}:
			case <-ctx.Done():
			}
			return
		}

		if from == 0 && result.Hits.Total.Value > 0 {
			select {
			case out <- scraper.SceneResult{Total: result.Hits.Total.Value}:
			case <-ctx.Done():
				return
			}
		}

		if len(result.Hits.Hits) == 0 {
			return
		}

		for _, hit := range result.Hits.Hits {
			id := strconv.Itoa(hit.Source.ItemID)

			if len(opts.KnownIDs) > 0 && opts.KnownIDs[id] {
				select {
				case out <- scraper.SceneResult{StoppedEarly: true}:
				case <-ctx.Done():
				}
				return
			}

			scene := hitToScene(hit.Source, studioURL, now)
			select {
			case out <- scraper.SceneResult{Scene: scene}:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (s *Scraper) search(ctx context.Context, query map[string]any) (*esResponse, error) {
	return s.searchURL(ctx, esEndpoint, query)
}

func (s *Scraper) searchURL(ctx context.Context, url string, query map[string]any) (*esResponse, error) {
	body, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}

	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:    url,
		Method: http.MethodPost,
		Body:   body,
		Headers: map[string]string{
			"Content-Type": "application/json",
			"Origin":       siteBase,
			"User-Agent":   httpx.UserAgentFirefox,
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result esResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &result, nil
}

type esResponse struct {
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []struct {
			Source esScene `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

type esScene struct {
	ID            string    `json:"id"`
	ItemID        int       `json:"itemId"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Img           string    `json:"img"`
	VideoTrailer  string    `json:"videoTrailer"`
	PublishedDate string    `json:"publishedDate"`
	Tags          []string  `json:"tags"`
	Models        []esModel `json:"models"`
	Site          struct {
		Name     string `json:"name"`
		NickName string `json:"nickName"`
	} `json:"site"`
}

type esModel struct {
	ID     string `json:"id"`
	ItemID int    `json:"itemId"`
	Name   string `json:"name"`
}

func hitToScene(src esScene, studioURL string, now time.Time) models.Scene {
	id := strconv.Itoa(src.ItemID)

	var performers []string
	for _, m := range src.Models {
		if m.Name != "" {
			performers = append(performers, m.Name)
		}
	}

	desc := src.Description
	desc = stripTagsRe.ReplaceAllString(desc, "")
	desc = html.UnescapeString(desc)
	desc = strings.TrimSpace(desc)

	scene := models.Scene{
		ID:          id,
		SiteID:      "mylf",
		StudioURL:   studioURL,
		Title:       src.Title,
		URL:         siteBase + "/videos/" + src.ID,
		Thumbnail:   src.Img,
		Preview:     src.VideoTrailer,
		Description: desc,
		Performers:  performers,
		Tags:        src.Tags,
		Date:        parseDate(src.PublishedDate),
		Studio:      src.Site.Name,
		ScrapedAt:   now,
	}
	scene.AddPrice(models.PriceSnapshot{Date: now, IsFree: false})
	return scene
}

func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func parseDate(s string) time.Time {
	t, err := time.Parse("2006-01-02T15:04:05", s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}
