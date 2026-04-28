package mydirtyhobby

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const (
	defaultSiteBase    = "https://www.mydirtyhobby.com"
	defaultContentBase = "https://www.mydirtyhobby.com"
	defaultPageSize    = 20
	defaultDelay       = 500 * time.Millisecond
)

// Scraper implements scraper.StudioScraper for MyDirtyHobby.
type Scraper struct {
	client      *http.Client
	siteBase    string
	contentBase string
	pageSize    int
}

func New() *Scraper {
	return &Scraper{
		client:      httpx.NewClient(30 * time.Second),
		siteBase:    defaultSiteBase,
		contentBase: defaultContentBase,
		pageSize:    defaultPageSize,
	}
}

func init() {
	scraper.Register(New())
}

// ---- StudioScraper interface ----

func (s *Scraper) ID() string { return "mydirtyhobby" }

func (s *Scraper) Patterns() []string {
	return []string{
		"mydirtyhobby.com/profil/{userId}-{username}",
		"mydirtyhobby.com/profil/{userId}-{username}/videos",
	}
}

// matchRe gates MatchesURL to only mydirtyhobby.com URLs.
var matchRe = regexp.MustCompile(`^https?://(?:www\.)?mydirtyhobby\.com/profil/\d+-`)

// profileRe extracts the user ID and nick from any URL containing /profil/{id}-{nick}.
var profileRe = regexp.MustCompile(`/profil/(\d+)-([^/?]+)`)

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	uid, nick, err := profileParams(studioURL)
	if err != nil {
		return nil, err
	}
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, uid, nick, opts, out)
	return out, nil
}

// ---- runner ----

func (s *Scraper) run(ctx context.Context, studioURL string, uid int, nick string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	delay := opts.Delay
	if delay == 0 {
		delay = defaultDelay
	}

	for page := 1; ; page++ {
		if ctx.Err() != nil {
			return
		}
		if page > 1 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}
		}

		items, total, totalPages, err := s.fetchPage(ctx, uid, page)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		if page == 1 && total > 0 {
			select {
			case out <- scraper.Progress(total):
			case <-ctx.Done():
				return
			}
		}

		now := time.Now().UTC()
		hitKnown := false
		for _, item := range items {
			id := strconv.Itoa(item.UVID)
			if len(opts.KnownIDs) > 0 && opts.KnownIDs[id] {
				hitKnown = true
				break
			}
			scene := toScene(studioURL, s.siteBase, uid, nick, item, now)
			select {
			case out <- scraper.Scene(scene):
			case <-ctx.Done():
				return
			}
		}

		if hitKnown || page >= totalPages || len(items) == 0 {
			if hitKnown {
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
			}
			return
		}
	}
}

// ---- API call ----

type listRequest struct {
	Page         int    `json:"page"`
	PageSize     int    `json:"pageSize"`
	UserID       int    `json:"user_id"`
	UserLanguage string `json:"user_language"`
}

type listResponse struct {
	Items      []mdhItem `json:"items"`
	Total      int       `json:"total"`
	Page       int       `json:"page"`
	TotalPages int       `json:"totalPages"`
}

type mdhItem struct {
	UID                 int     `json:"u_id"`
	UVID                int     `json:"uv_id"`
	Nick                string  `json:"nick"`
	Title               string  `json:"title"`
	Description         string  `json:"description"`
	Thumbnail           string  `json:"thumbnail"`
	Price               string  `json:"price"`
	HasDiscount         bool    `json:"hasDiscount"`
	ReducedPercent      *int    `json:"reducedPercent"`
	VotesAverage        float64 `json:"votesAverage"`
	VotesCount          int     `json:"votesCount"`
	Duration            string  `json:"duration"`
	LatestPictureChange string  `json:"latestPictureChange"`
	Language            string  `json:"language"`
}

func (s *Scraper) fetchPage(ctx context.Context, uid, page int) ([]mdhItem, int, int, error) {
	body, err := json.Marshal(listRequest{
		Page:         page,
		PageSize:     s.pageSize,
		UserID:       uid,
		UserLanguage: "en",
	})
	if err != nil {
		return nil, 0, 0, err
	}

	u := s.contentBase + "/content/api/v2/videos"
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:  u,
		Body: body,
		Headers: map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
			"Cookie":       "AGEGATEPASSED=1",
			"User-Agent":   httpx.UserAgentFirefox,
		},
	})
	if err != nil {
		return nil, 0, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	var lr listResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return nil, 0, 0, fmt.Errorf("decoding response: %w", err)
	}
	return lr.Items, lr.Total, lr.TotalPages, nil
}

// ---- mapping ----

func toScene(studioURL, siteBase string, uid int, nick string, item mdhItem, now time.Time) models.Scene {
	regularCents, priceErr := strconv.ParseFloat(item.Price, 64)
	regular := regularCents / 100

	var discounted float64
	var discountPct int
	if item.HasDiscount && item.ReducedPercent != nil {
		discountPct = *item.ReducedPercent
		discounted = regular * float64(100-discountPct) / 100
	}

	scene := models.Scene{
		ID:          strconv.Itoa(item.UVID),
		SiteID:      "mydirtyhobby",
		StudioURL:   studioURL,
		Title:       item.Title,
		URL:         fmt.Sprintf("%s/profil/%d-%s/videos/%d", siteBase, uid, url.PathEscape(nick), item.UVID),
		Date:        parseDate(item.LatestPictureChange),
		Description: item.Description,
		Thumbnail:   item.Thumbnail,
		Studio:      item.Nick,
		Duration:    parseDuration(item.Duration),
		Likes:       item.VotesCount,
		ScrapedAt:   now,
	}

	if priceErr == nil {
		scene.AddPrice(models.PriceSnapshot{
			Date:            now,
			Regular:         regular,
			Discounted:      discounted,
			IsFree:          regular == 0,
			IsOnSale:        item.HasDiscount,
			DiscountPercent: discountPct,
		})
	}

	return scene
}

// ---- helpers ----

func profileParams(studioURL string) (int, string, error) {
	m := profileRe.FindStringSubmatch(studioURL)
	if m == nil {
		return 0, "", fmt.Errorf("cannot extract profile ID from %q", studioURL)
	}
	uid, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, "", fmt.Errorf("invalid profile ID %q: %w", m[1], err)
	}
	// Strip any trailing path segment from slug (e.g., "/videos").
	nick := strings.SplitN(m[2], "/", 2)[0]
	return uid, nick, nil
}

// parseDuration converts "MM:SS" or "HH:MM:SS" to seconds.
func parseDuration(s string) int {
	parts := strings.Split(s, ":")
	total := 0
	for _, p := range parts {
		n, _ := strconv.Atoi(p)
		total = total*60 + n
	}
	return total
}

// parseDate parses ISO 8601 timestamps with timezone offset.
func parseDate(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, _ = time.Parse("2006-01-02T15:04:05-07:00", s)
	}
	return t.UTC()
}
