// Package dezyred scrapes Dezyred (dezyred.com), a VR interactive-games studio
// in the VRBangers family. The whole catalog (~51 games) is served by a single
// open JSON endpoint at /api/games. Performer IDs on each game resolve to names
// via a companion /api/models endpoint, which the scraper fetches once and
// caches as an id→name map. There is no pagination — one fetch returns every
// game — so the runner emits each game as a Scene.
package dezyred

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const (
	siteID     = "dezyred"
	studioName = "Dezyred"
)

// siteBase is a var (not const) so tests can point it at a local httptest server.
var siteBase = "https://dezyred.com"

// Scraper implements scraper.StudioScraper for Dezyred.
type Scraper struct {
	Client *http.Client
}

// New constructs a Dezyred scraper.
func New() *Scraper {
	return &Scraper{Client: httpx.NewClient(30 * time.Second)}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		"dezyred.com",
		"dezyred.com/games/{slug}",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?dezyred\.com`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// ---- JSON shapes ----

type game struct {
	ID          string         `json:"id"`
	PageURL     string         `json:"pageUrl"`
	Title       string         `json:"title"`
	Annotation  string         `json:"annotation"`
	Description string         `json:"description"`
	Rating      float64        `json:"rating"`
	CreatedAt   string         `json:"createdAt"`
	Models      []int          `json:"models"`
	Categories  []gameCategory `json:"categories"`
	Posters     gamePosters    `json:"posters"`
}

type gameCategory struct {
	Title string `json:"title"`
}

type gamePosters struct {
	Item     string `json:"item"`
	ListItem string `json:"listItem"`
}

type model struct {
	ID        string `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

func (s *Scraper) run(ctx context.Context, studioURL string, _ scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()

	scraper.Debugf(1, "%s: fetching model directory", siteID)
	modelNames, err := s.fetchModels(ctx)
	if err != nil {
		// Model names are a nice-to-have; log and continue with an empty map.
		scraper.Debugf(1, "%s: model directory failed: %v", siteID, err)
		modelNames = map[int]string{}
	}

	scraper.Debugf(1, "%s: fetching games", siteID)
	games, err := s.fetchGames(ctx)
	if err != nil {
		select {
		case out <- scraper.Error(fmt.Errorf("games: %w", err)):
		case <-ctx.Done():
		}
		return
	}
	scraper.Debugf(1, "%s: %d games", siteID, len(games))

	select {
	case out <- scraper.Progress(len(games)):
	case <-ctx.Done():
		return
	}

	for _, g := range games {
		scene := toScene(studioURL, g, modelNames, now)
		select {
		case out <- scraper.Scene(scene):
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scraper) fetchGames(ctx context.Context) ([]game, error) {
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL:     siteBase + "/api/games",
		Headers: httpx.BrowserHeaders(httpx.UserAgentChrome),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var games []game
	if err := httpx.DecodeJSON(resp.Body, &games); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return games, nil
}

func (s *Scraper) fetchModels(ctx context.Context) (map[int]string, error) {
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL:     siteBase + "/api/models",
		Headers: httpx.BrowserHeaders(httpx.UserAgentChrome),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var mods []model
	if err := httpx.DecodeJSON(resp.Body, &mods); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	out := make(map[int]string, len(mods))
	for _, m := range mods {
		var id int
		if _, err := fmt.Sscanf(m.ID, "%d", &id); err != nil {
			continue
		}
		name := strings.TrimSpace(m.FirstName + " " + m.LastName)
		if name != "" {
			out[id] = name
		}
	}
	return out, nil
}

var tagStripRe = regexp.MustCompile(`<[^>]+>`)

func toScene(studioURL string, g game, modelNames map[int]string, now time.Time) models.Scene {
	id := strings.TrimSpace(g.ID)
	slug := strings.TrimPrefix(g.PageURL, "/games/")
	if id == "" {
		id = slug
	}

	url := g.PageURL
	if strings.HasPrefix(url, "/") {
		url = siteBase + url
	}

	scene := models.Scene{
		ID:          id,
		SiteID:      siteID,
		StudioURL:   studioURL,
		Title:       html.UnescapeString(strings.TrimSpace(g.Title)),
		URL:         url,
		Description: cleanText(firstNonEmpty(g.Description, g.Annotation)),
		Studio:      studioName,
		Date:        parseDate(g.CreatedAt),
		ScrapedAt:   now,
	}

	for _, id := range g.Models {
		if name := modelNames[id]; name != "" {
			scene.Performers = append(scene.Performers, name)
		}
	}

	for _, c := range g.Categories {
		if t := strings.TrimSpace(c.Title); t != "" {
			scene.Categories = append(scene.Categories, html.UnescapeString(t))
		}
	}

	if thumb := firstNonEmpty(g.Posters.Item, g.Posters.ListItem); thumb != "" {
		scene.Thumbnail = thumb
	}

	return scene
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func cleanText(s string) string {
	s = tagStripRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}

// parseDate parses createdAt timestamps like "2026-06-19T21:23:50.000000Z".
func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{"2006-01-02T15:04:05.000000Z07:00", time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
