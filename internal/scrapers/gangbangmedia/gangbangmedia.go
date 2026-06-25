// Package gangbangmedia scrapes Gangbang Media Germany (p-p-p.tv). Detail
// pages are login-gated, so the scraper parses the public /videos/list
// listing only. Each listing card carries the video id, slug, duration,
// and thumbnail; the title and performers are derived from the card title
// text / slug. The listing shows a relative German date that cannot be
// converted reliably, so Date is left zero (the listing is newest-first).
package gangbangmedia

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/parseutil"
	"github.com/Wasylq/FSS/scraper"
)

const (
	siteBase = "https://p-p-p.tv"
	siteID   = "gangbangmedia"
)

type Scraper struct{ client *http.Client }

func New() *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second)}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{"p-p-p.tv", "p-p-p.tv/videos/list"}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?p-p-p\.tv`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		cards, err := s.fetchCards(ctx, fmt.Sprintf("%s/videos/list?page=%d", siteBase, page))
		if err != nil {
			// The listing 404s past the last page — treat as a clean stop.
			var se *httpx.StatusError
			if errors.As(err, &se) && se.StatusCode == http.StatusNotFound {
				scraper.Debugf(1, "%s: page %d returned 404, ending listing", siteID, page)
				return scraper.PageResult{Done: true}, nil
			}
			return scraper.PageResult{}, err
		}
		scenes := make([]models.Scene, 0, len(cards))
		for _, c := range cards {
			if sc, ok := toScene(studioURL, c, now); ok {
				scenes = append(scenes, sc)
			}
		}
		return scraper.PageResult{Scenes: scenes}, nil
	})
}

// cardSplitRe splits the page on each listing card's anchor. Each resulting
// part begins at the slug, e.g. `alicia-dark-und-diana-love" data-controller...`.
var cardSplitRe = regexp.MustCompile(`<a class="h-100" href="/video/`)

func (s *Scraper) fetchCards(ctx context.Context, pageURL string) ([]string, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
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
	parts := cardSplitRe.Split(string(body), -1)
	if len(parts) <= 1 {
		return nil, nil
	}
	return parts[1:], nil
}

var (
	slugRe     = regexp.MustCompile(`^([a-z0-9][\w-]*)"`)
	videoIDRe  = regexp.MustCompile(`data-thumb-videoid-value="(\d+)"`)
	imgSrcRe   = regexp.MustCompile(`<img class="card-img-top" src="([^"]+)"`)
	titleRe    = regexp.MustCompile(`(?s)<strong>\s*(.*?)\s*</strong>`)
	durationRe = regexp.MustCompile(`fa-clock-o[^>]*></i>\s*([0-9:]+)`)
	teilRe     = regexp.MustCompile(`(?i)-und-teil-[\w-]+$|-teil-[\w-]+$`)
)

func toScene(studioURL, card string, now time.Time) (models.Scene, bool) {
	idm := videoIDRe.FindStringSubmatch(card)
	if idm == nil {
		return models.Scene{}, false
	}
	sm := slugRe.FindStringSubmatch(card)
	if sm == nil {
		return models.Scene{}, false
	}
	slug := sm[1]

	scene := models.Scene{
		ID:         idm[1],
		SiteID:     siteID,
		StudioURL:  studioURL,
		URL:        siteBase + "/video/" + slug,
		Studio:     "Gangbang Media Germany",
		Performers: performersFromSlug(slug),
		ScrapedAt:  now,
	}

	if t := titleRe.FindStringSubmatch(card); t != nil {
		scene.Title = html.UnescapeString(strings.TrimSpace(t[1]))
	}
	if scene.Title == "" {
		scene.Title = titleFromSlug(slug)
	}

	if im := imgSrcRe.FindStringSubmatch(card); im != nil {
		scene.Thumbnail = absURL(html.UnescapeString(im[1]))
	}
	if d := durationRe.FindStringSubmatch(card); d != nil {
		scene.Duration = parseutil.ParseDurationColon(d[1])
	}

	return scene, true
}

func absURL(u string) string {
	if strings.HasPrefix(u, "/") {
		return siteBase + u
	}
	return u
}

// performersFromSlug derives performer names from the scene slug. Performers
// are joined by "und" in the slug; a trailing "-teil-N" part marker is dropped.
// e.g. "alicia-dark-und-diana-love-teil-2" -> ["Alicia Dark", "Diana Love"].
func performersFromSlug(slug string) []string {
	slug = teilRe.ReplaceAllString(slug, "")
	parts := strings.Split(slug, "-und-")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if name := titleCase(strings.ReplaceAll(p, "-", " ")); name != "" {
			out = append(out, name)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// titleFromSlug builds a display title from the slug, dropping the "und" joiner.
func titleFromSlug(slug string) string {
	words := strings.FieldsFunc(slug, func(r rune) bool { return r == '-' })
	kept := words[:0]
	for _, w := range words {
		if strings.EqualFold(w, "und") {
			continue
		}
		kept = append(kept, w)
	}
	return titleCase(strings.Join(kept, " "))
}

func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		r := []rune(w)
		r[0] = []rune(strings.ToUpper(string(r[0])))[0]
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}
