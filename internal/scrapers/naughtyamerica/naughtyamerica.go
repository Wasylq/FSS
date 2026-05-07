package naughtyamerica

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

const apiBase = "https://api.naughtyapi.com/tools/scenes/scenes"

type Scraper struct {
	client *http.Client
	apiURL string
}

func New() *Scraper {
	return &Scraper{
		client: httpx.NewClient(30 * time.Second),
		apiURL: apiBase,
	}
}

func init() {
	scraper.Register(New())
}

func (s *Scraper) ID() string { return "naughtyamerica" }

func (s *Scraper) Patterns() []string {
	return []string{
		"naughtyamerica.com",
		"naughtyamericavr.com",
		"myfriendshotmom.com",
		"mysistershotfriend.com",
		"tonightsgirlfriend.com",
		"thundercock.com",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?(?:naughtyamerica(?:vr)?|myfriendshotmom|mysistershotfriend|tonightsgirlfriend|thundercock)\.com`)

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

type apiResponse struct {
	CurrentPage int     `json:"current_page"`
	LastPage    int     `json:"last_page"`
	Total       int     `json:"total"`
	PerPage     int     `json:"per_page"`
	Data        []scene `json:"data"`
}

type scene struct {
	ID            int                 `json:"id"`
	Title         string              `json:"title"`
	Length        int                 `json:"length"`
	PublishedDate string              `json:"published_date"`
	SceneURL      string              `json:"scene_url"`
	POV           string              `json:"pov"`
	Degrees       int                 `json:"degrees"`
	Synopsis      string              `json:"synopsis"`
	Tags          []string            `json:"tags"`
	Performers    map[string][]string `json:"performers"`
	Trailers      map[string]string   `json:"trailers"`
	SiteName      string              `json:"site_name"`
	RawPromoVideo json.RawMessage     `json:"promo_video_data"`
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

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

		resp, err := s.fetchPage(ctx, page)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		if page == 1 && resp.Total > 0 {
			select {
			case out <- scraper.Progress(resp.Total):
			case <-ctx.Done():
				return
			}
		}

		if len(resp.Data) == 0 {
			return
		}

		now := time.Now().UTC()
		for _, item := range resp.Data {
			id := strconv.Itoa(item.ID)
			if opts.KnownIDs[id] {
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
				return
			}
			select {
			case out <- scraper.Scene(toScene(studioURL, item, now)):
			case <-ctx.Done():
				return
			}
		}

		if page >= resp.LastPage {
			return
		}
	}
}

func (s *Scraper) fetchPage(ctx context.Context, page int) (*apiResponse, error) {
	u := fmt.Sprintf("%s?page=%d", s.apiURL, page)
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: u,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentFirefox,
			"Accept":     "application/json",
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result apiResponse
	if err := httpx.DecodeJSON(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &result, nil
}

func parsePromoVideo(raw json.RawMessage) map[string]string {
	if len(raw) == 0 || raw[0] == '[' {
		return nil
	}
	var m map[string]string
	_ = json.Unmarshal(raw, &m)
	return m
}

func toScene(studioURL string, s scene, now time.Time) models.Scene {
	performers := allPerformers(s.Performers)
	promos := parsePromoVideo(s.RawPromoVideo)
	sc := models.Scene{
		ID:          strconv.Itoa(s.ID),
		SiteID:      "naughtyamerica",
		StudioURL:   studioURL,
		Title:       s.Title,
		URL:         s.SceneURL,
		Description: s.Synopsis,
		Thumbnail:   thumbnailURL(s.Trailers, promos),
		Preview:     previewURL(s.Trailers),
		Duration:    s.Length,
		Performers:  performers,
		Tags:        s.Tags,
		Studio:      s.SiteName,
		ScrapedAt:   now,
	}
	if t, err := time.Parse("2006-01-02 15:04:05", s.PublishedDate); err == nil {
		sc.Date = t.UTC()
	}
	if s.Degrees >= 180 {
		sc.Tags = appendIfMissing(sc.Tags, "VR")
	}
	return sc
}

func allPerformers(m map[string][]string) []string {
	var out []string
	for _, names := range m {
		out = append(out, names...)
	}
	return out
}

// thumbnailURL constructs image URL from trailer/promo video paths.
// Promo pattern: ".../promo/{prefix}/{slug}/{prefix}{slug}_..." → "{prefix}/{slug}"
// Trailer pattern: ".../naughtycdn.com/{prefix}/trailers/.../{prefix}{slug}trailer_..." → "{prefix}/{slug}"
//
//	or VR variant: ".../trailers/vr/{prefix}{slug}/..." → "{prefix}/{slug}"
var promoPathRe = regexp.MustCompile(`/promo/([^/]+)/([^/]+)/`)
var trailerPrefixRe = regexp.MustCompile(`naughtycdn\.com/(?:nonsecure/)?([^/]+)/trailers/`)
var trailerVRSlugRe = regexp.MustCompile(`/trailers/vr/([^/]+)/`)
var trailerFileRe = regexp.MustCompile(`/([^/]+?)(?:teaser|trailer)[^/]*$`)

func thumbnailURL(trailers, promos map[string]string) string {
	for _, u := range promos {
		if m := promoPathRe.FindStringSubmatch(u); m != nil {
			return fmt.Sprintf("https://images4.naughtycdn.com/cms/nacmscontent/v1/scenes/%s/%s/scene/horizontal/1279x852c.jpg", m[1], m[2])
		}
	}
	for _, u := range trailers {
		var prefix, slug string
		if m := trailerPrefixRe.FindStringSubmatch(u); m != nil {
			prefix = m[1]
		}
		if prefix == "" {
			continue
		}
		if m := trailerVRSlugRe.FindStringSubmatch(u); m != nil {
			slug = strings.TrimPrefix(m[1], prefix)
		} else if m := trailerFileRe.FindStringSubmatch(u); m != nil {
			slug = strings.TrimPrefix(m[1], prefix)
		}
		if slug != "" {
			return fmt.Sprintf("https://images4.naughtycdn.com/cms/nacmscontent/v1/scenes/%s/%s/scene/horizontal/1279x852c.jpg", prefix, slug)
		}
	}
	return ""
}

func previewURL(trailers map[string]string) string {
	for _, key := range []string{"trailer_720", "vrdesktophd", "vrdesktopsd", "180_sbs", "smartphonevr60", "smartphonevr30"} {
		if u, ok := trailers[key]; ok {
			return u
		}
	}
	for _, u := range trailers {
		return u
	}
	return ""
}

func appendIfMissing(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}
