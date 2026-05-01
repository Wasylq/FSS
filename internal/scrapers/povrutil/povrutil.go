package povrutil

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

type SiteConfig struct {
	ID       string
	Studio   string
	SiteBase string
	Patterns []string
	MatchRe  *regexp.Regexp
}

type Scraper struct {
	Cfg    SiteConfig
	Client *http.Client
}

func New(cfg SiteConfig) *Scraper {
	return &Scraper{
		Cfg:    cfg,
		Client: httpx.NewClient(30 * time.Second),
	}
}

func (s *Scraper) ID() string               { return s.Cfg.ID }
func (s *Scraper) Patterns() []string       { return s.Cfg.Patterns }
func (s *Scraper) MatchesURL(u string) bool { return s.Cfg.MatchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, opts, out)
	return out, nil
}

// exportVideo is one entry from /export/videos.json.
type exportVideo struct {
	URL    string   `json:"url"`
	Length int      `json:"length"`
	Title  string   `json:"title"`
	Tags   []string `json:"tags"`
	Thumb  string   `json:"thumb"`
	Actors []string `json:"actors"`
}

var sceneIDRe = regexp.MustCompile(`-(\d+)$`)

func extractID(rawURL string) string {
	rawURL = strings.TrimSuffix(rawURL, "/")
	if m := sceneIDRe.FindStringSubmatch(rawURL); m != nil {
		return m[1]
	}
	return ""
}

func (s *Scraper) run(ctx context.Context, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	export, err := s.fetchExport(ctx)
	if err != nil {
		select {
		case out <- scraper.Error(fmt.Errorf("export: %w", err)):
		case <-ctx.Done():
		}
		return
	}

	lookup := make(map[string]exportVideo, len(export))
	for _, v := range export {
		path := urlPath(v.URL)
		if path != "" {
			lookup[path] = v
		}
	}

	if len(export) > 0 {
		select {
		case out <- scraper.Progress(len(export)):
		case <-ctx.Done():
			return
		}
	}

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

		cards, err := s.fetchListingPage(ctx, page)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}
		if len(cards) == 0 {
			return
		}

		stoppedEarly := false
		for _, c := range cards {
			id := extractID(c.path)
			if id == "" {
				continue
			}
			if len(opts.KnownIDs) > 0 && opts.KnownIDs[id] {
				stoppedEarly = true
				break
			}

			scene := s.buildScene(c, lookup[c.path], id, now)
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
	}
}

func (s *Scraper) buildScene(c listingCard, ev exportVideo, id string, now time.Time) models.Scene {
	sc := models.Scene{
		ID:         id,
		SiteID:     s.Cfg.ID,
		StudioURL:  s.Cfg.SiteBase,
		Title:      c.title,
		URL:        s.Cfg.SiteBase + c.path,
		Date:       c.date,
		Performers: c.performers,
		Studio:     s.Cfg.Studio,
		ScrapedAt:  now,
	}

	if ev.URL != "" {
		sc.Tags = ev.Tags
		sc.Duration = ev.Length
		sc.Thumbnail = ev.Thumb
		if len(sc.Performers) == 0 {
			sc.Performers = ev.Actors
		}
	}

	return sc
}

// ---- export fetch ----

func (s *Scraper) fetchExport(ctx context.Context) ([]exportVideo, error) {
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL: s.Cfg.SiteBase + "/export/videos.json",
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentFirefox,
			"Accept":     "application/json",
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var videos []exportVideo
	if err := json.NewDecoder(resp.Body).Decode(&videos); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return videos, nil
}

// ---- listing page parsing ----

type listingCard struct {
	path       string
	title      string
	performers []string
	date       time.Time
}

var (
	cardRe      = regexp.MustCompile(`(?s)cards-list__item card\s*">(.*?)</div>\s*</div>\s*</div>\s*</div>`)
	cardLinkRe  = regexp.MustCompile(`<a href="(/[^"]+)" class="card__video`)
	cardTitleRe = regexp.MustCompile(`card__h">([^<]+)`)
	cardPerfsRe = regexp.MustCompile(`card__links">(.*?)</div>`)
	perfNameRe  = regexp.MustCompile(`<a[^>]*>([^<]+)</a>`)
	cardDateRe  = regexp.MustCompile(`card__date">.*?</svg>\s*(\d{1,2}\s\w+,\s\d{4})`)
)

func parseListingCards(body []byte) []listingCard {
	matches := cardRe.FindAllSubmatch(body, -1)
	cards := make([]listingCard, 0, len(matches))
	for _, m := range matches {
		block := m[1]
		c := listingCard{}

		if lm := cardLinkRe.FindSubmatch(block); lm != nil {
			c.path = string(lm[1])
		}
		if tm := cardTitleRe.FindSubmatch(block); tm != nil {
			c.title = strings.TrimSpace(string(tm[1]))
		}
		if pm := cardPerfsRe.FindSubmatch(block); pm != nil {
			for _, nm := range perfNameRe.FindAllSubmatch(pm[1], -1) {
				name := strings.TrimSpace(string(nm[1]))
				if name != "" {
					c.performers = append(c.performers, name)
				}
			}
		}
		if dm := cardDateRe.FindSubmatch(block); dm != nil {
			if t, err := time.Parse("2 January, 2006", string(dm[1])); err == nil {
				c.date = t.UTC()
			}
		}

		if c.path != "" {
			cards = append(cards, c)
		}
	}
	return cards
}

func (s *Scraper) fetchListingPage(ctx context.Context, page int) ([]listingCard, error) {
	u := fmt.Sprintf("%s/?o=d&p=%d", s.Cfg.SiteBase, page)
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL: u,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentFirefox,
			"Accept":     "text/html",
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseListingCards(body), nil
}

func urlPath(rawURL string) string {
	i := strings.Index(rawURL, "://")
	if i < 0 {
		return ""
	}
	rest := rawURL[i+3:]
	j := strings.Index(rest, "/")
	if j < 0 {
		return ""
	}
	return rest[j:]
}
