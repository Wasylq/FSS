package teamskeetutil

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
	esBase   = "https://tours-store.psmcdn.net"
	pageSize = 30
)

type SiteConfig struct {
	SiteID   string
	Domain   string
	SiteBase string
	Index    string // ES index name (e.g. "ts_network", "mylf_bundle")
}

type Scraper struct {
	client    *http.Client
	Config    SiteConfig
	esBaseURL string // override for testing; defaults to esBase
}

func NewScraper(cfg SiteConfig) *Scraper {
	return &Scraper{
		client: httpx.NewClient(30 * time.Second),
		Config: cfg,
	}
}

func (s *Scraper) Run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	kind, value := classifyURL(studioURL)
	baseQuery := buildQuery(kind, value)
	baseQuery["size"] = pageSize
	now := time.Now().UTC()

	var searchAfter []json.RawMessage
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

		if searchAfter != nil {
			baseQuery["search_after"] = searchAfter
		}

		result, err := s.search(ctx, baseQuery)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		if page == 0 && result.Hits.Total.Value > 0 {
			select {
			case out <- scraper.Progress(result.Hits.Total.Value):
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
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
				return
			}

			scene := hitToScene(hit.Source, studioURL, s.Config.SiteBase, now)
			select {
			case out <- scraper.Scene(scene):
			case <-ctx.Done():
				return
			}
		}

		lastHit := result.Hits.Hits[len(result.Hits.Hits)-1]
		if len(lastHit.Sort) == 0 {
			return
		}
		searchAfter = lastHit.Sort
	}
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
		must = append(must, map[string]any{"match_phrase": map[string]any{"tags": tag}})
	}

	return map[string]any{
		"query": map[string]any{
			"bool": map[string]any{"must": must},
		},
		"sort": []any{
			map[string]any{"publishedDate": map[string]any{"order": "desc"}},
			map[string]any{"itemId": map[string]any{"order": "desc"}},
		},
	}
}

func (s *Scraper) search(ctx context.Context, query map[string]any) (*esResponse, error) {
	base := s.esBaseURL
	if base == "" {
		base = esBase
	}
	url := base + "/" + s.Config.Index + "/_search"
	return s.searchWithURL(ctx, url, query)
}

func (s *Scraper) searchWithURL(ctx context.Context, url string, query map[string]any) (*esResponse, error) {
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
			"Origin":       s.Config.SiteBase,
			"User-Agent":   httpx.UserAgentFirefox,
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result esResponse
	if err := httpx.DecodeJSON(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &result, nil
}

type esResponse struct {
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []esHit `json:"hits"`
	} `json:"hits"`
}

type esHit struct {
	Source esScene           `json:"_source"`
	Sort   []json.RawMessage `json:"sort"`
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

var stripTagsRe = regexp.MustCompile(`<[^>]+>`)

func hitToScene(src esScene, studioURL, siteBase string, now time.Time) models.Scene {
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
		SiteID:      src.Site.NickName,
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

func parseDate(s string) time.Time {
	t, err := time.Parse("2006-01-02T15:04:05", s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}
