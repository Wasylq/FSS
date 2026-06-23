// Package dungeoncorputil scrapes the Dungeon Corp BDSM network (SocietySM,
// Cumbots, Fucking Dungeon, PerfectSlave, Strict Restraint, DungeonCorp). All
// brands are served by one PHP app at www.dungeoncorp.com, selected by a site
// code:
//
//	https://www.dungeoncorp.com/?page=updates&site={CODE}&p={N}
//
// Each .updatebox listing card carries the full per-scene metadata (title,
// performers, date, duration, thumbnail), so no detail fetch is needed.
package dungeoncorputil

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

const networkBase = "https://www.dungeoncorp.com"

type SiteConfig struct {
	ID       string
	Studio   string
	Code     string // dungeoncorp site code: SSM, CUM, FUD, PER, STR, DUN
	Patterns []string
	MatchRe  *regexp.Regexp
}

type Scraper struct {
	cfg    SiteConfig
	Client *http.Client
}

func New(cfg SiteConfig) *Scraper {
	return &Scraper{
		cfg:    cfg,
		Client: httpx.NewClient(30 * time.Second),
	}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func (s *Scraper) ID() string               { return s.cfg.ID }
func (s *Scraper) Patterns() []string       { return s.cfg.Patterns }
func (s *Scraper) MatchesURL(u string) bool { return s.cfg.MatchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	cardStartRe = regexp.MustCompile(`<a href="[^"]+" title="[^"]*" class="updatebox"`)
	hrefRe      = regexp.MustCompile(`<a href="([^"]+)" title="([^"]*)" class="updatebox"`)
	updateIDRe  = regexp.MustCompile(`data-update-id="(\d+)"`)
	imgRe       = regexp.MustCompile(`<img[^>]+src="([^"]+)"`)
	modelRe     = regexp.MustCompile(`href="/\?page=models&model=[^"]*">([^<]+)</a>`)
	dateRe      = regexp.MustCompile(`fa-clock"></i>\s*(\d{2}/\d{2}/\d{4})`)
	durationRe  = regexp.MustCompile(`fa-video"></i>\s*(\d+)\s*min`)
)

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, s.cfg.ID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := fmt.Sprintf("%s/?page=updates&site=%s&p=%d", networkBase, s.cfg.Code, page)
		cards, err := s.fetchCards(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}
		scenes := make([]models.Scene, 0, len(cards))
		for _, c := range cards {
			if sc, ok := s.toScene(studioURL, c, now); ok {
				scenes = append(scenes, sc)
			}
		}
		return scraper.PageResult{Scenes: scenes}, nil
	})
}

func (s *Scraper) fetchCards(ctx context.Context, pageURL string) ([]string, error) {
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL:     pageURL,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, err
	}
	body, err := func() ([]byte, error) {
		defer func() { _ = resp.Body.Close() }()
		return httpx.ReadBody(resp.Body)
	}()
	if err != nil {
		return nil, err
	}
	text := string(body)
	locs := cardStartRe.FindAllStringIndex(text, -1)
	cards := make([]string, 0, len(locs))
	for i, loc := range locs {
		end := len(text)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		cards = append(cards, text[loc[0]:end])
	}
	return cards, nil
}

func (s *Scraper) toScene(studioURL, card string, now time.Time) (models.Scene, bool) {
	m := hrefRe.FindStringSubmatch(card)
	if m == nil {
		return models.Scene{}, false
	}
	scene := models.Scene{
		SiteID:    s.cfg.ID,
		StudioURL: studioURL,
		URL:       m[1],
		Title:     html.UnescapeString(strings.TrimSpace(m[2])),
		Studio:    s.cfg.Studio,
		ScrapedAt: now,
	}
	if id := updateIDRe.FindStringSubmatch(card); id != nil {
		scene.ID = id[1]
	} else {
		scene.ID = slugFromURL(m[1])
	}
	if img := imgRe.FindStringSubmatch(card); img != nil {
		scene.Thumbnail = img[1]
	}
	for _, pm := range modelRe.FindAllStringSubmatch(card, -1) {
		if name := strings.TrimSpace(html.UnescapeString(pm[1])); name != "" {
			scene.Performers = append(scene.Performers, name)
		}
	}
	if d := dateRe.FindStringSubmatch(card); d != nil {
		if t, err := time.Parse("01/02/2006", d[1]); err == nil {
			scene.Date = t.UTC()
		}
	}
	if dur := durationRe.FindStringSubmatch(card); dur != nil {
		var mins int
		_, _ = fmt.Sscanf(dur[1], "%d", &mins)
		scene.Duration = mins * 60
	}
	return scene, true
}

func slugFromURL(u string) string {
	u = strings.TrimRight(u, "/")
	if i := strings.LastIndex(u, "/"); i >= 0 {
		return u[i+1:]
	}
	return u
}
