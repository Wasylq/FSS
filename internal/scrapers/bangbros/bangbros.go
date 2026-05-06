package bangbros

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/internal/scrapers/ayloutil"
	"github.com/Wasylq/FSS/scraper"
)

var config = ayloutil.SiteConfig{
	SiteID:     "bangbros",
	SiteBase:   "https://www.bangbros.com",
	StudioName: "Bang Bros",
}

type Scraper struct {
	aylo *ayloutil.Scraper
}

func New() *Scraper {
	return &Scraper{aylo: ayloutil.NewScraper(config)}
}

func init() {
	scraper.Register(New())
}

func (s *Scraper) ID() string { return config.SiteID }

func (s *Scraper) Patterns() []string {
	return []string{
		"bangbros.com",
		"bangbros.com/model/{id}/{slug}",
		"bangbros.com/category/{id}/{slug}",
		"bangbros.com/category/{slug}",
		"bangbros.com/site/{id}/{slug}",
		"bangbros.com/websites/{slug}",
		"bangbros.com/series/{id}/{slug}",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?bangbros\.com`)

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

// websiteSlugRe matches /websites/{slug}.
var websiteSlugRe = regexp.MustCompile(`(?i)/websites/([^/?#]+)`)

// categorySlugRe matches /category/{slug} where the slug starts with a letter (no numeric ID).
var categorySlugRe = regexp.MustCompile(`(?i)/category/([a-zA-Z][^/?#]*)`)

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	resolved, err := s.resolveSlugURL(ctx, studioURL)
	if err != nil {
		return nil, err
	}
	out := make(chan scraper.SceneResult)
	go s.aylo.Run(ctx, resolved, opts, out)
	return out, nil
}

// resolveSlugURL translates BangBros slug-only URLs into numeric-ID URLs that ayloutil understands.
//   - /websites/{slug} → /site/{id}/{slug} via collection search
//   - /category/{slug} → /category/{id}/{slug} via tag lookup
//
// Other URLs (already have numeric IDs) pass through unchanged.
func (s *Scraper) resolveSlugURL(ctx context.Context, studioURL string) (string, error) {
	if m := websiteSlugRe.FindStringSubmatch(studioURL); m != nil {
		slug := m[1]
		id, err := s.resolveCollectionID(ctx, slug)
		if err != nil {
			return "", err
		}
		return "https://www.bangbros.com/site/" + strconv.Itoa(id) + "/" + strings.ToLower(slug), nil
	}
	if m := categorySlugRe.FindStringSubmatch(studioURL); m != nil {
		slug := m[1]
		id, err := s.resolveTagID(ctx, slug)
		if err != nil {
			return "", err
		}
		return "https://www.bangbros.com/category/" + strconv.Itoa(id) + "/" + strings.ToLower(slug), nil
	}
	return studioURL, nil
}

// resolveCollectionID finds the numeric collection ID for a BangBros sub-site slug (e.g. "MomIsHorny").
// It searches for scenes from the sub-site and matches the collection name against the slug.
func (s *Scraper) resolveCollectionID(ctx context.Context, slug string) (int, error) {
	token, err := s.aylo.FetchToken(ctx)
	if err != nil {
		return 0, fmt.Errorf("resolving collection %q: %w", slug, err)
	}
	apiURL := s.aylo.APIHost + "/v2/releases?type=scene&limit=5&search=" + url.QueryEscape(slug)
	resp, err := httpx.Do(ctx, s.aylo.Client, httpx.Request{
		URL: apiURL,
		Headers: map[string]string{
			"Instance": token,
			"Accept":   "application/json",
		},
	})
	if err != nil {
		return 0, fmt.Errorf("resolving collection %q: %w", slug, err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Result []struct {
			Collections []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"collections"`
		} `json:"result"`
	}
	if err := httpx.DecodeJSON(resp.Body, &result); err != nil {
		return 0, fmt.Errorf("decoding collection search for %q: %w", slug, err)
	}
	for _, scene := range result.Result {
		for _, c := range scene.Collections {
			if strings.EqualFold(c.Name, slug) {
				return c.ID, nil
			}
		}
	}
	return 0, fmt.Errorf("collection %q not found in search results", slug)
}

// resolveTagID finds the numeric tag ID for a BangBros category slug (e.g. "brunette").
func (s *Scraper) resolveTagID(ctx context.Context, slug string) (int, error) {
	token, err := s.aylo.FetchToken(ctx)
	if err != nil {
		return 0, fmt.Errorf("resolving tag %q: %w", slug, err)
	}
	apiURL := s.aylo.APIHost + "/v2/tags?name=" + url.QueryEscape(slug) + "&limit=1"
	resp, err := httpx.Do(ctx, s.aylo.Client, httpx.Request{
		URL: apiURL,
		Headers: map[string]string{
			"Instance": token,
			"Accept":   "application/json",
		},
	})
	if err != nil {
		return 0, fmt.Errorf("resolving tag %q: %w", slug, err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Result []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"result"`
	}
	if err := httpx.DecodeJSON(resp.Body, &result); err != nil {
		return 0, fmt.Errorf("decoding tag search for %q: %w", slug, err)
	}
	if len(result.Result) == 0 {
		return 0, fmt.Errorf("tag %q not found", slug)
	}
	return result.Result[0].ID, nil
}
