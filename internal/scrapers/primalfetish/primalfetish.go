// Package primalfetish scrapes the Primal Fetish Network paysites
// (primalfetishnetwork.com), a custom PHP CMS hosting several fetish paysites
// that all share one listing template. Every scene's metadata (title,
// performers, date, duration, thumbnail) is published on the listing cards, so
// the scraper is listing-only — no per-scene detail fetch is required.
//
// Each paysite is registered as its own FSS scraper (its own SiteID). The
// per-paysite listing lives at /paysites/{id}/{slug}/videos/ (page 1) and
// /paysites/{id}/{slug}/videos/page{N}.html (page N >= 2). Listings are
// server-side filtered to the paysite and sorted newest-first, so KnownIDs
// early-stop by scene ID works.
package primalfetish

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

const siteBase = "https://primalfetishnetwork.com"

// SiteConfig describes one Primal Fetish paysite.
type SiteConfig struct {
	SiteID     string // FSS site identifier, e.g. "primalstaboofamily"
	PaysiteID  int    // numeric CMS paysite id, e.g. 14
	Slug       string // cosmetic URL slug, e.g. "primals-taboo-relations"
	StudioName string // human-readable studio name
}

var sites = []SiteConfig{
	{SiteID: "primalstaboofamily", PaysiteID: 14, Slug: "primals-taboo-relations", StudioName: "Primal's Taboo Family Relations"},
	{SiteID: "primalscosplay", PaysiteID: 32, Slug: "primals-cosplay", StudioName: "Primal's Cosplay"},
	{SiteID: "primalsfootfantasies", PaysiteID: 25, Slug: "primals-foot-fantasies", StudioName: "Primal's FOOT FANTASIES"},
	{SiteID: "primalsfootjobs", PaysiteID: 4, Slug: "primals-footjobs", StudioName: "Primal's FOOTJOBS"},
	{SiteID: "primalshandjobs", PaysiteID: 23, Slug: "primals-handjobs", StudioName: "Primal's HANDJOBS"},
	{SiteID: "primalspovfamilylust", PaysiteID: 12, Slug: "primals-pov-family-lust", StudioName: "Primal's POV Family Lust"},
	{SiteID: "primalswrestlingsex", PaysiteID: 16, Slug: "primals-wrestling-sex", StudioName: "Primals Wrestling Sex"},
	{SiteID: "primaleroticmassage", PaysiteID: 1, Slug: "erotic-massage-master", StudioName: "Erotic Massage Institute"},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(New(cfg))
	}
}

// Scraper scrapes a single Primal Fetish paysite.
type Scraper struct {
	cfg     SiteConfig
	matchRe *regexp.Regexp
	Client  *http.Client
}

var _ scraper.StudioScraper = (*Scraper)(nil)

// New constructs a scraper for one paysite config.
func New(cfg SiteConfig) *Scraper {
	return &Scraper{
		cfg: cfg,
		// Match the paysite by its numeric id in the URL path, or by its slug.
		matchRe: regexp.MustCompile(fmt.Sprintf(
			`primalfetishnetwork\.com/paysites/%d/|primalfetishnetwork\.com/paysites/\d+/%s\b`,
			cfg.PaysiteID, regexp.QuoteMeta(cfg.Slug))),
		Client: httpx.NewClient(30 * time.Second),
	}
}

func (s *Scraper) ID() string { return s.cfg.SiteID }

func (s *Scraper) Patterns() []string {
	return []string{
		fmt.Sprintf("primalfetishnetwork.com/paysites/%d/%s/videos/", s.cfg.PaysiteID, s.cfg.Slug),
	}
}

func (s *Scraper) MatchesURL(u string) bool { return s.matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	scraper.Debugf(1, "primalfetish: scraping paysite %d (%s) %s", s.cfg.PaysiteID, s.cfg.SiteID, s.cfg.Slug)

	scraper.Paginate(ctx, opts, s.cfg.SiteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		body, err := s.fetchPage(ctx, s.pageURL(page))
		if err != nil {
			return scraper.PageResult{}, err
		}
		scenes := s.parseListing(body, studioURL)
		// An empty page (no real video cards) signals the end of the listing.
		return scraper.PageResult{
			Scenes: scenes,
			Done:   len(scenes) == 0,
		}, nil
	})
}

// pageURL builds the listing URL for a 1-based page number.
func (s *Scraper) pageURL(page int) string {
	base := fmt.Sprintf("%s/paysites/%d/%s/videos/", siteBase, s.cfg.PaysiteID, s.cfg.Slug)
	if page <= 1 {
		return base
	}
	return fmt.Sprintf("%spage%d.html", base, page)
}

var (
	cardRe     = regexp.MustCompile(`<div class="main__videoElement`)
	titleRe    = regexp.MustCompile(`(?s)<a href="(https://primalfetishnetwork\.com/video/[^"]*?-(\d+)\.html)"[^>]*class="main__videoTitle"[^>]*>\s*(.*?)\s*</a>`)
	modelRe    = regexp.MustCompile(`<a class="video__listModel" title="([^"]*)"`)
	dateRe     = regexp.MustCompile(`(?s)<div class="date">\s*Date:\s*(.*?)\s*</div>`)
	timeRe     = regexp.MustCompile(`(?s)<div class="time">\s*Time:\s*(.*?)\s*</div>`)
	thumbRe    = regexp.MustCompile(`<img[^>]*src="(https://cdn\.primalfetishnetwork\.com/thumbs/[^"]+)"`)
	dateLayout = "2 Jan 2006"
)

// parseListing extracts every scene card from a listing page.
func (s *Scraper) parseListing(body []byte, studioURL string) []models.Scene {
	page := string(body)
	now := time.Now().UTC()

	locs := cardRe.FindAllStringIndex(page, -1)
	scenes := make([]models.Scene, 0, len(locs))

	for i, loc := range locs {
		end := len(page)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		block := page[loc[0]:end]

		tm := titleRe.FindStringSubmatch(block)
		if tm == nil {
			// JS template / placeholder card (href="#") — not a real scene.
			continue
		}
		scene := models.Scene{
			ID:        tm[2],
			SiteID:    s.cfg.SiteID,
			StudioURL: studioURL,
			Studio:    s.cfg.StudioName,
			Title:     strings.TrimSpace(html.UnescapeString(tm[3])),
			URL:       tm[1],
			ScrapedAt: now,
		}

		for _, m := range modelRe.FindAllStringSubmatch(block, -1) {
			name := strings.TrimSpace(html.UnescapeString(m[1]))
			if name != "" {
				scene.Performers = append(scene.Performers, name)
			}
		}

		if m := dateRe.FindStringSubmatch(block); m != nil {
			raw := parseutil.StripOrdinalSuffix(strings.TrimSpace(m[1]))
			if t, err := time.Parse(dateLayout, raw); err == nil {
				scene.Date = t.UTC()
			}
		}

		if m := timeRe.FindStringSubmatch(block); m != nil {
			scene.Duration = parseutil.ParseDurationColon(strings.TrimSpace(m[1]))
		}

		if m := thumbRe.FindStringSubmatch(block); m != nil {
			scene.Thumbnail = m[1]
		}

		scenes = append(scenes, scene)
	}
	return scenes
}

func (s *Scraper) fetchPage(ctx context.Context, url string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL:     url,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}
