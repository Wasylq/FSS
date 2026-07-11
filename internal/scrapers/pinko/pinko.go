// Package pinko scrapes the Pinko network sites — Pinko TGirls
// (pinkotgirls.com) and Pinko Club (pinkoclub.com) — which run the same
// custom PHP CMS and differ only in their scene-path prefix.
//
// The /new-video.php?next={N} listing is ID-descending and carries the scene
// ID, English title and CDN thumbnail on each `link-photo-home` card. A
// worker pool then fetches each detail page to enrich the description (from
// og:description) and the cast (from /trans-star/ or /pornostar/ links).
//
// NOTE: the site exposes neither a publish date nor a duration anywhere, so
// Scene.Date and Scene.Duration are intentionally left zero. The static
// JSON-LD VideoObject on detail pages is a copy-paste artifact (it references
// unrelated scenes) and is deliberately ignored.
package pinko

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/parseutil"
	"github.com/Wasylq/FSS/scraper"
)

// SiteConfig describes one Pinko network site.
type SiteConfig struct {
	SiteID       string // stable scraper identifier
	Domain       string // bare hostname for URL matching
	Base         string // scheme + host, no trailing slash
	StudioName   string // human-readable studio name
	DetailPrefix string // scene-path prefix, e.g. "/videotrans/"
}

var sites = []SiteConfig{
	{SiteID: "pinkotgirls", Domain: "pinkotgirls.com", Base: "https://www.pinkotgirls.com", StudioName: "Pinko TGirls", DetailPrefix: "/videotrans/"},
	{SiteID: "pinkoclub", Domain: "pinkoclub.com", Base: "https://www.pinkoclub.com", StudioName: "Pinko Club", DetailPrefix: "/video-porno-italiani/"},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(New(cfg))
	}
}

// Scraper implements scraper.StudioScraper for one Pinko site.
type Scraper struct {
	cfg      SiteConfig
	Client   *http.Client
	matchRe  *regexp.Regexp
	cardIDRe *regexp.Regexp
}

var _ scraper.StudioScraper = (*Scraper)(nil)

// New builds a Scraper for the given site config.
func New(cfg SiteConfig) *Scraper {
	dom := regexp.QuoteMeta(cfg.Domain)
	prefix := regexp.QuoteMeta(strings.Trim(cfg.DetailPrefix, "/"))
	return &Scraper{
		cfg:      cfg,
		Client:   httpx.NewClient(30 * time.Second),
		matchRe:  regexp.MustCompile(`^https?://(?:www\.)?` + dom + `(?:/|$)`),
		cardIDRe: regexp.MustCompile(`/` + prefix + `/(\d+)-`),
	}
}

func (s *Scraper) ID() string { return s.cfg.SiteID }

func (s *Scraper) Patterns() []string {
	return []string{
		s.cfg.Domain,
		s.cfg.Domain + "/new-video.php",
	}
}

func (s *Scraper) MatchesURL(u string) bool { return s.matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// card is one entry parsed from a listing page.
type card struct {
	id    string
	url   string
	title string
	thumb string
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	workers := opts.Workers
	if workers <= 0 {
		workers = 4
	}

	work := make(chan card)
	var wg sync.WaitGroup
	scraper.Debugf(1, "%s: fetching detail pages with %d workers", s.cfg.SiteID, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range work {
				scene, err := s.fetchDetail(ctx, c, studioURL, opts.Delay)
				if err != nil {
					select {
					case out <- scraper.Error(err):
					case <-ctx.Done():
						return
					}
					continue
				}
				select {
				case out <- scraper.Scene(scene):
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(work)
		s.enqueueListing(ctx, opts, out, work)
	}()

	wg.Wait()
}

func (s *Scraper) enqueueListing(ctx context.Context, opts scraper.ListOpts, out chan<- scraper.SceneResult, work chan<- card) {
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
		pageURL := s.listURL(page)
		scraper.Debugf(1, "%s: fetching listing page %d (%s)", s.cfg.SiteID, page, pageURL)
		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}
		cards := s.parseListing(body)
		if len(cards) == 0 {
			scraper.Debugf(1, "%s: page %d empty, stopping", s.cfg.SiteID, page)
			return
		}
		for _, c := range cards {
			if opts.KnownIDs[c.id] {
				scraper.Debugf(1, "%s: hit known ID %s, stopping early", s.cfg.SiteID, c.id)
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
				return
			}
			select {
			case work <- c:
			case <-ctx.Done():
				return
			}
		}
	}
}

// listURL builds the 1-indexed listing page URL. The page parameter is
// `next=` (not page/p).
func (s *Scraper) listURL(page int) string {
	return fmt.Sprintf("%s/new-video.php?next=%d", s.cfg.Base, page)
}

// listingCardRe captures href, title and thumbnail src from a card anchor.
var listingCardRe = regexp.MustCompile(`<a class="link-photo-home" href="([^"]+)" title="([^"]*)">\s*<img[^>]*?\bsrc="([^"]+)"`)

// parseListing extracts the ID, detail URL, title and thumbnail from each
// card on a listing page, de-duplicating by ID.
func (s *Scraper) parseListing(body []byte) []card {
	page := string(body)
	ms := listingCardRe.FindAllStringSubmatch(page, -1)
	cards := make([]card, 0, len(ms))
	seen := map[string]bool{}
	for _, m := range ms {
		href := m[1]
		idm := s.cardIDRe.FindStringSubmatch(href)
		if idm == nil {
			continue
		}
		id := idm[1]
		if seen[id] {
			continue
		}
		seen[id] = true
		cards = append(cards, card{
			id:    id,
			url:   s.absURL(href),
			title: strings.TrimSpace(html.UnescapeString(m[2])),
			thumb: m[3],
		})
	}
	return cards
}

type detailData struct {
	title       string
	description string
	image       string
	performers  []string
}

var (
	// titoloH4Re isolates the cast <h4> inside the scene's title block so
	// performer links are scoped to the scene cast (not page chrome).
	titoloH4Re = regexp.MustCompile(`(?s)<div class="titolo-video">.*?<h4>(.*?)</h4>`)
	// performerRe matches cast anchors: /trans-star/ (TGirls) or
	// /pornostar/ (Club).
	performerRe = regexp.MustCompile(`<a href="/(?:trans-star|pornostar)/[^"]+"[^>]*>([^<]+)</a>`)
)

// parseDetail pulls the title/description/image from OpenGraph tags and the
// cast from the title block. The static JSON-LD VideoObject is ignored.
func parseDetail(body []byte) detailData {
	og := parseutil.OpenGraph(body)
	d := detailData{
		title:       strings.TrimSpace(html.UnescapeString(og["og:title"])),
		description: strings.TrimSpace(html.UnescapeString(og["og:description"])),
		image:       strings.TrimSpace(html.UnescapeString(og["og:image"])),
	}
	if m := titoloH4Re.FindSubmatch(body); m != nil {
		for _, pm := range performerRe.FindAllStringSubmatch(string(m[1]), -1) {
			name := strings.TrimSpace(html.UnescapeString(pm[1]))
			if name != "" {
				d.performers = append(d.performers, name)
			}
		}
	}
	return d
}

func (s *Scraper) fetchDetail(ctx context.Context, c card, studioURL string, delay time.Duration) (models.Scene, error) {
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return models.Scene{}, ctx.Err()
		}
	}

	scene := models.Scene{
		ID:        c.id,
		SiteID:    s.cfg.SiteID,
		StudioURL: studioURL,
		Title:     c.title,
		URL:       c.url,
		Thumbnail: c.thumb,
		Studio:    s.cfg.StudioName,
		ScrapedAt: time.Now().UTC(),
	}

	body, err := s.fetchPage(ctx, c.url)
	if err != nil {
		return models.Scene{}, fmt.Errorf("detail %s: %w", c.id, err)
	}
	d := parseDetail(body)
	if d.title != "" {
		scene.Title = d.title
	}
	scene.Description = d.description
	if scene.Thumbnail == "" && d.image != "" {
		scene.Thumbnail = d.image
	}
	scene.Performers = d.performers
	return scene, nil
}

func (s *Scraper) absURL(u string) string {
	if strings.HasPrefix(u, "http") {
		return u
	}
	if strings.HasPrefix(u, "/") {
		return s.cfg.Base + u
	}
	return s.cfg.Base + "/" + u
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
