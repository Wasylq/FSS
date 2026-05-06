package wankitnowutil

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

type SiteConfig struct {
	ID       string
	Domain   string
	Studio   string
	Patterns []string
	MatchRe  *regexp.Regexp
	BaseURL  string // optional override for testing; defaults to "https://www." + Domain
}

type Scraper struct {
	cfg    SiteConfig
	client *http.Client
}

func New(cfg SiteConfig) *Scraper {
	return &Scraper{
		cfg:    cfg,
		client: httpx.NewClient(30 * time.Second),
	}
}

func (s *Scraper) ID() string         { return s.cfg.ID }
func (s *Scraper) Patterns() []string { return s.cfg.Patterns }
func (s *Scraper) MatchesURL(u string) bool {
	return s.cfg.MatchRe.MatchString(u)
}

func (s *Scraper) siteBase() string {
	if s.cfg.BaseURL != "" {
		return s.cfg.BaseURL
	}
	return "https://www." + s.cfg.Domain
}

func (s *Scraper) ListScenes(ctx context.Context, _ string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, opts, out)
	return out, nil
}

type nextDataResponse struct {
	PageProps struct {
		Contents struct {
			Total      int         `json:"total"`
			TotalPages int         `json:"total_pages"`
			Data       []sceneJSON `json:"data"`
		} `json:"contents"`
	} `json:"pageProps"`
}

type sceneJSON struct {
	ID              int              `json:"id"`
	Title           string           `json:"title"`
	Slug            string           `json:"slug"`
	Description     string           `json:"description"`
	PublishDate     string           `json:"publish_date"`
	SecondsDuration int              `json:"seconds_duration"`
	Thumb           string           `json:"thumb"`
	Models          []string         `json:"models"`
	Tags            []string         `json:"tags"`
	Site            string           `json:"site"`
	SiteDomain      string           `json:"site_domain"`
	Rating          float64          `json:"rating"`
	ModelsSlugs     []modelSlugEntry `json:"models_slugs"`
}

type modelSlugEntry struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

var buildIDRe = regexp.MustCompile(`"buildId"\s*:\s*"([^"]+)"`)

func (s *Scraper) fetchBuildID(ctx context.Context) (string, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: s.siteBase() + "/",
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentChrome,
		},
	})
	if err != nil {
		return "", fmt.Errorf("fetch homepage: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read homepage: %w", err)
	}

	m := buildIDRe.FindSubmatch(body)
	if m == nil {
		return "", fmt.Errorf("buildId not found in homepage")
	}
	return string(m[1]), nil
}

func (s *Scraper) fetchPage(ctx context.Context, buildID string, page int) (*nextDataResponse, error) {
	u := fmt.Sprintf("%s/_next/data/%s/videos.json?page=%d", s.siteBase(), buildID, page)
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: u,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentChrome,
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var data nextDataResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode page %d: %w", page, err)
	}
	return &data, nil
}

func (s *Scraper) run(ctx context.Context, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	buildID, err := s.fetchBuildID(ctx)
	if err != nil {
		select {
		case out <- scraper.Error(err):
		case <-ctx.Done():
		}
		return
	}

	siteBase := s.siteBase()
	now := time.Now().UTC()

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

		data, err := s.fetchPage(ctx, buildID, page)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		contents := data.PageProps.Contents

		if page == 1 && contents.Total > 0 {
			select {
			case out <- scraper.Progress(contents.Total):
			case <-ctx.Done():
				return
			}
		}

		if len(contents.Data) == 0 {
			return
		}

		stoppedEarly := false
		for _, sc := range contents.Data {
			id := strconv.Itoa(sc.ID)
			if len(opts.KnownIDs) > 0 && opts.KnownIDs[id] {
				stoppedEarly = true
				break
			}

			scene := toScene(sc, s.cfg.ID, siteBase, s.cfg.Studio, now)
			select {
			case out <- scraper.Scene(scene):
			case <-ctx.Done():
				return
			}
		}

		if stoppedEarly {
			select {
			case out <- scraper.StoppedEarly():
			case <-ctx.Done():
			}
			return
		}

		if page >= contents.TotalPages {
			return
		}
	}
}

func toScene(sc sceneJSON, siteID, siteBase, studio string, now time.Time) models.Scene {
	var date time.Time
	if sc.PublishDate != "" {
		if t, err := time.Parse("2006/01/02 15:04:05", sc.PublishDate); err == nil {
			date = t.UTC()
		}
	}

	performers := make([]string, 0, len(sc.Models))
	for _, m := range sc.Models {
		name := strings.TrimSpace(m)
		if name != "" {
			performers = append(performers, normalizePerformer(name))
		}
	}

	return models.Scene{
		ID:          strconv.Itoa(sc.ID),
		SiteID:      siteID,
		StudioURL:   siteBase,
		Title:       sc.Title,
		Description: sc.Description,
		URL:         siteBase + "/videos/" + sc.Slug,
		Thumbnail:   sc.Thumb,
		Date:        date,
		Duration:    sc.SecondsDuration,
		Performers:  performers,
		Tags:        sc.Tags,
		Studio:      studio,
		ScrapedAt:   now,
	}
}

func normalizePerformer(name string) string {
	words := strings.Fields(name)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	return strings.Join(words, " ")
}
