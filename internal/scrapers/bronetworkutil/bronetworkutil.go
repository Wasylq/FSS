// Package bronetworkutil scrapes the Pinstripe Media Group / "Bro Network"
// gay paysites (MENatPLAY, MASQULIN, The Bro Network, Men of Montréal,
// Amateur Gay POV). They share one custom PHP CMS whose listing cards carry
// the full per-scene metadata (title, performers, date, duration, thumbnail),
// so no detail-page fetch is needed.
//
// Listing:  {SiteBase}/categories/{Slug}_{page}_d.html  (date-descending)
// Scene:    {SiteBase}/updates/{Name}.html
package bronetworkutil

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
	"github.com/Wasylq/FSS/parseutil"
	"github.com/Wasylq/FSS/scraper"
)

type SiteConfig struct {
	ID       string
	Studio   string
	SiteBase string // e.g. "https://menatplay.com" — no trailing slash
	Slug     string // listing category slug, e.g. "movies", "videos", "masqulin"
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
	cardSplitRe   = regexp.MustCompile(`<div class="updateDetails">`)
	hrefRe        = regexp.MustCompile(`<a\s+href="([^"]+)"`)
	posterRe      = regexp.MustCompile(`poster="([^"]+)"`)
	imgThumbRe    = regexp.MustCompile(`src="([^"]+contentthumbs[^"]+)"`)
	contentIDRe   = regexp.MustCompile(`/contentthumbs/\d+/\d+/(\d+)`)
	titleRe       = regexp.MustCompile(`<h4>\s*([^<]+?)\s*</h4>`)
	modelsBlockRe = regexp.MustCompile(`(?s)<span class="tour_update_models">(.*?)</span>`)
	modelLinkRe   = regexp.MustCompile(`>([^<]+)</a>`)
	availDateRe   = regexp.MustCompile(`class="availdate"[^>]*>\s*([^<]+?)\s*</span>`)
	durationRe    = regexp.MustCompile(`(\d{1,2}:\d{2})\s*min`)
	// listing/category URL the operator may pass directly
	listingURLRe = regexp.MustCompile(`/categories/([A-Za-z0-9_-]+?)_\d+_[a-z]\.html`)
)

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	slug := s.cfg.Slug
	if m := listingURLRe.FindStringSubmatch(studioURL); m != nil {
		slug = m[1]
		scraper.Debugf(1, "%s: using category slug from URL: %s", s.cfg.ID, slug)
	}

	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, s.cfg.ID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := fmt.Sprintf("%s/categories/%s_%d_d.html", s.cfg.SiteBase, slug, page)
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
	parts := cardSplitRe.Split(string(body), -1)
	if len(parts) <= 1 {
		return nil, nil
	}
	return parts[1:], nil // drop the pre-first-card preamble
}

func (s *Scraper) toScene(studioURL, card string, now time.Time) (models.Scene, bool) {
	m := hrefRe.FindStringSubmatch(card)
	if m == nil {
		return models.Scene{}, false
	}
	url := m[1]
	if !strings.Contains(url, "/updates/") {
		return models.Scene{}, false
	}
	if !strings.HasPrefix(url, "http") {
		url = s.cfg.SiteBase + "/" + strings.TrimPrefix(url, "/")
	}

	scene := models.Scene{
		SiteID:    s.cfg.ID,
		StudioURL: studioURL,
		URL:       url,
		Studio:    s.cfg.Studio,
		ScrapedAt: now,
	}

	// ID: prefer the numeric content id, fall back to the URL slug.
	// Thumbnail comes from a <video poster> or a lazy <img src> card.
	thumb := ""
	if poster := posterRe.FindStringSubmatch(card); poster != nil {
		thumb = poster[1]
	} else if img := imgThumbRe.FindStringSubmatch(card); img != nil {
		thumb = img[1]
	}
	if thumb != "" {
		scene.Thumbnail = thumb
		if id := contentIDRe.FindStringSubmatch(thumb); id != nil {
			scene.ID = id[1]
		}
	}
	if scene.ID == "" {
		scene.ID = slugFromURL(url)
	}

	if t := titleRe.FindStringSubmatch(card); t != nil {
		scene.Title = html.UnescapeString(strings.TrimSpace(t[1]))
	}
	if scene.Title == "" {
		scene.Title = strings.ReplaceAll(slugFromURL(url), "-", " ")
	}

	if blk := modelsBlockRe.FindStringSubmatch(card); blk != nil {
		for _, pm := range modelLinkRe.FindAllStringSubmatch(blk[1], -1) {
			if name := strings.TrimSpace(html.UnescapeString(pm[1])); name != "" {
				scene.Performers = append(scene.Performers, name)
			}
		}
	}

	dates := availDateRe.FindAllStringSubmatch(card, -1)
	if len(dates) > 0 {
		if d, err := time.Parse("Jan 2, 2006", strings.TrimSpace(dates[0][1])); err == nil {
			scene.Date = d.UTC()
		}
	}
	if dur := durationRe.FindStringSubmatch(card); dur != nil {
		scene.Duration = parseutil.ParseDurationColon(dur[1])
	}

	return scene, true
}

func slugFromURL(u string) string {
	u = strings.TrimSuffix(u, ".html")
	if i := strings.LastIndex(u, "/"); i >= 0 {
		return u[i+1:]
	}
	return u
}
