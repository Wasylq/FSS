// Package d2passutil scrapes the D2Pass "bifrost" JSON platform shared by
// 1Pondo, 10musume, Pacopacomama and Muramura.
//
// Each site is a Vue SPA whose HTML is an empty shell, but the API behind it is
// entirely public — no auth, no cookie, no Referer check. The listing endpoint
// returns the same record shape as the per-movie detail endpoint (identical key
// set), so a listing page alone yields fully populated scenes and no detail
// fetch is needed.
//
//	/dyn/phpauto/movie_lists/list_newest_{offset}.json
//	  -> {"TotalRows": N, "SplitSize": 50, "Rows": [...]}
//
// {offset} is a row offset that must be a multiple of SplitSize — the server
// 404s on any other value, so pagination steps by SplitSize rather than by page
// number. Rows come back strictly newest-first, so the KnownIDs early-stop
// applies.
//
// Titles, descriptions and tags exist in both Japanese and English. English is
// preferred where present, since DescEn in particular is frequently empty and
// has to fall back to the Japanese text.
package d2passutil

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

// defaultSplitSize is the server's page size. The real value is echoed in each
// response; this is only the fallback for a response that omits it.
const defaultSplitSize = 50

// SiteConfig defines one D2Pass site.
type SiteConfig struct {
	SiteID     string
	Domain     string
	StudioName string
}

// Scraper handles listing and scene building for a D2Pass instance.
type Scraper struct {
	cfg      SiteConfig
	Client   *http.Client
	SiteBase string
}

// New creates a Scraper for the given site configuration.
func New(cfg SiteConfig) *Scraper {
	return &Scraper{
		cfg:      cfg,
		Client:   httpx.NewClient(30 * time.Second),
		SiteBase: "https://" + cfg.Domain,
	}
}

// Config returns the site configuration.
func (s *Scraper) Config() SiteConfig { return s.cfg }

// ListPage is one page of the movie-list API.
type ListPage struct {
	TotalRows int     `json:"TotalRows"`
	SplitSize int     `json:"SplitSize"`
	Rows      []Movie `json:"Rows"`
}

// Movie is a single scene record. The listing and detail endpoints return the
// same shape, so this covers both.
type Movie struct {
	MovieID     string   `json:"MovieID"`
	MetaMovieID int      `json:"MetaMovieID"`
	Title       string   `json:"Title"`
	TitleEn     string   `json:"TitleEn"`
	Desc        string   `json:"Desc"`
	DescEn      string   `json:"DescEn"`
	Release     string   `json:"Release"`
	Duration    int      `json:"Duration"`
	ActressesJa []string `json:"ActressesJa"`
	ActressesEn []string `json:"ActressesEn"`
	UCNAME      []string `json:"UCNAME"`
	UCNAMEEn    []string `json:"UCNAMEEn"`
	Series      *string  `json:"Series"`
	SeriesEn    *string  `json:"SeriesEn"`
	ThumbUltra  string   `json:"ThumbUltra"`
	ThumbHigh   string   `json:"ThumbHigh"`
	ThumbMed    string   `json:"ThumbMed"`
	MovieThumb  string   `json:"MovieThumb"`
}

// Run walks the newest-first listing and emits every scene.
func (s *Scraper) Run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	now := time.Now().UTC()
	split := defaultSplitSize

	scraper.Paginate(ctx, opts, s.cfg.SiteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		// The API keys off a row offset, not a page number, and rejects any
		// offset that is not a multiple of SplitSize.
		offset := (page - 1) * split

		lp, err := s.FetchPage(ctx, offset)
		if err != nil {
			return scraper.PageResult{}, err
		}
		if lp.SplitSize > 0 {
			split = lp.SplitSize
		}
		if len(lp.Rows) == 0 {
			return scraper.PageResult{Done: true}, nil
		}

		scenes := make([]models.Scene, 0, len(lp.Rows))
		for _, m := range lp.Rows {
			if m.MovieID == "" {
				continue
			}
			scenes = append(scenes, s.ToScene(studioURL, m, now))
		}
		return scraper.PageResult{
			Scenes: scenes,
			Total:  lp.TotalRows,
			Done:   lp.TotalRows > 0 && offset+len(lp.Rows) >= lp.TotalRows,
			// A page whose rows all lacked a MovieID is not the end of the
			// listing.
			Continue: len(scenes) == 0,
		}, nil
	})
}

// FetchPage retrieves one page of the listing at the given row offset.
func (s *Scraper) FetchPage(ctx context.Context, offset int) (ListPage, error) {
	u := fmt.Sprintf("%s/dyn/phpauto/movie_lists/list_newest_%d.json", s.SiteBase, offset)
	scraper.Debugf(1, "%s: fetching offset %d", s.cfg.SiteID, offset)

	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL:     u,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return ListPage{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	var lp ListPage
	if err := httpx.DecodeJSON(resp.Body, &lp); err != nil {
		return ListPage{}, fmt.Errorf("decoding %s offset %d: %w", s.cfg.SiteID, offset, err)
	}
	return lp, nil
}

// ToScene converts an API record into a models.Scene.
func (s *Scraper) ToScene(studioURL string, m Movie, now time.Time) models.Scene {
	scene := models.Scene{
		ID:          m.MovieID,
		SiteID:      s.cfg.SiteID,
		StudioURL:   studioURL,
		Title:       preferEn(m.TitleEn, m.Title),
		URL:         fmt.Sprintf("%s/movies/%s/", s.SiteBase, m.MovieID),
		Description: preferEn(m.DescEn, m.Desc),
		Studio:      s.cfg.StudioName,
		Thumbnail:   firstNonEmpty(m.ThumbUltra, m.ThumbHigh, m.ThumbMed, m.MovieThumb),
		Duration:    m.Duration,
		Performers:  preferEnList(m.ActressesEn, m.ActressesJa),
		Tags:        preferEnList(m.UCNAMEEn, m.UCNAME),
		ScrapedAt:   now,
	}

	if d, err := time.Parse("2006-01-02", strings.TrimSpace(m.Release)); err == nil {
		scene.Date = d.UTC()
	}
	if v := derefNonEmpty(m.SeriesEn, m.Series); v != "" {
		scene.Series = v
	}
	return scene
}

// MovieURL builds the public page URL for a scene.
func (s *Scraper) MovieURL(movieID string) string {
	return fmt.Sprintf("%s/movies/%s/", s.SiteBase, movieID)
}

// ---- helpers ----

// preferEn returns the English value when it carries text, else the Japanese
// one. DescEn especially is often an empty string rather than absent.
func preferEn(en, ja string) string {
	if v := strings.TrimSpace(en); v != "" {
		return v
	}
	return strings.TrimSpace(ja)
}

func preferEnList(en, ja []string) []string {
	src := en
	if len(nonEmpty(en)) == 0 {
		src = ja
	}
	return nonEmpty(src)
}

func nonEmpty(in []string) []string {
	var out []string
	for _, v := range in {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

func derefNonEmpty(vals ...*string) string {
	for _, v := range vals {
		if v == nil {
			continue
		}
		if t := strings.TrimSpace(*v); t != "" && !strings.EqualFold(t, "null") {
			return t
		}
	}
	return ""
}

// ParseOffset extracts the row offset from a list URL, for tests and debugging.
func ParseOffset(u string) (int, bool) {
	const marker = "list_newest_"
	i := strings.LastIndex(u, marker)
	if i < 0 {
		return 0, false
	}
	rest := strings.TrimSuffix(u[i+len(marker):], ".json")
	n, err := strconv.Atoi(rest)
	if err != nil {
		return 0, false
	}
	return n, true
}
