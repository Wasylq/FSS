// Package feetondemand scrapes the public Videos catalogue from five
// Feet on Demand sister tours that share the same custom AJAX-loaded
// template: Goddess Foot Domination, Jerk to My Feet, Foot Fetish Car
// Dates, Foot Fetish Affiliates, Goddess Brianna.
//
// Each tour page at `{base}/?mb=VmlkZW9zfHw=&p={offset}` (decodes
// `Videos||`) is a thin shell whose `<div id="mainbody">` is populated
// at runtime by a jQuery `.load()` call against a hashed inner URL
// `content/pages/{hash}.list.htm`. The hash isn't reproducible from
// the offset alone, so the scraper fetches the outer page first,
// extracts the AJAX URL, then fetches the inner page where the card
// grid actually lives.
//
// Card markup on the inner page:
//
//	<div class='col-lg-3 col-md-6 col-sm-12 img-portfolio'>
//	  <a href='#' data-toggle='modal' data-target='#pop_{sceneID}'>
//	    <img class='img-responsive thumbvideo'
//	         src='{base}/content/art/videos/{sceneID}.jpg'
//	         alt='{Title}'>
//	  </a>
//	  <h4>
//	    <a href='#' data-toggle='modal' data-target='#pop_{sceneID}'>{Title}</a>
//	  </h4>
//	  <p><strong>Model: </strong>
//	    <a href="?page=Models&id={modID}">{Performer}</a>
//	  </p>
//	</div>
//
// `{sceneID}` is a 14-character alphanumeric like `h3k3b9s9l9v2g2`.
// Detail pages don't exist — every scene plays in a Bootstrap modal on
// the same page — so we synthesise the URL as
// `{base}/?mb=VmlkZW9zfHw=#pop_{sceneID}` to give each scene a stable
// anchor.
//
// Pagination: 20 cards per page on goddessfootdomination, 15-16 on the
// other tours; the outer page emits paginator links of the form
// `p={offset}` and the highest is the last-page offset. We walk in
// increments of 20 from 0 to last (the last page may have fewer cards;
// stopping when the inner page yields 0 covers that).
//
// **The other 7 Feet on Demand sister sites are not covered** — five
// (feetpov, footfetishpetite, goddessfootworship, goddessfootjobs,
// fetishcustoms) are marketing splashes whose nav links all redirect
// to the join page, two (footslaveauditions returning 404,
// goddessbrianna redirect loop on root) are effectively dead. The
// parent `feetondemand.com` redirects to an AI-generated `manus.space`
// page that's not a real catalogue.
package feetondemand

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const (
	studioName   = "Feet on Demand"
	videosMb     = "VmlkZW9zfHw=" // base64 of "Videos||"
	cardsPerPage = 20             // pagination increment used by the tour's paginator
)

// SiteConfig describes one Feet on Demand sister tour.
type SiteConfig struct {
	ID       string
	BaseURL  string // e.g. "https://www.goddessfootdomination.com" — no trailing slash
	SiteName string
	Patterns []string
	MatchRe  *regexp.Regexp
}

type Scraper struct {
	cfg    SiteConfig
	client *http.Client
}

func New(cfg SiteConfig) *Scraper {
	return &Scraper{
		cfg:    cfg,
		client: httpx.NewClient(30 * time.Second),
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

// ---- parsing ----

var (
	// outerAjaxRe extracts the AJAX content URL the jQuery `.load()`
	// call on the outer page points at.
	outerAjaxRe = regexp.MustCompile(`\$\("#mainbody"\)\.load\("([^"]+)"\)`)
	// outerMaxPageRe matches every `p={N}` reference on the outer
	// page; the highest wins as the last-page offset.
	outerMaxPageRe = regexp.MustCompile(`p=(\d+)`)
	// innerCardStartRe matches the opening div of one card block in
	// the AJAX-loaded list. Card body runs to the next card or end of
	// document.
	innerCardStartRe = regexp.MustCompile(`<div class='[^']*img-portfolio[^']*'>`)
	// innerSceneIDRe — `data-target='#pop_{sceneID}'`. The sceneID is
	// an alphanumeric ~14-char hash.
	innerSceneIDRe = regexp.MustCompile(`data-target='#pop_([A-Za-z0-9]+)'`)
	// innerThumbRe — the thumbnail anchor's `<img src=…>` URL. The
	// path component `/content/art/videos/{ID}.jpg` should match the
	// sceneID found above but we don't enforce it (some templates use
	// a different thumb filename for the same scene).
	innerThumbRe = regexp.MustCompile(`<img\s[^>]*src='([^']+/content/art/videos/[^']+\.jpg)'`)
	// innerTitleRe — title text in the `<h4><a>…</a></h4>` block.
	innerTitleRe = regexp.MustCompile(`(?s)<h4[^>]*>\s*<a[^>]*>([^<]+)</a>\s*</h4>`)
	// innerPerformerRe — `<a href="?page=Models&id=…">{Name}</a>`.
	innerPerformerRe = regexp.MustCompile(`<a\s+href="(?:[^"]*)\?page=Models&(?:amp;)?id=[^"]+">([^<]+)</a>`)
)

type card struct {
	id        string
	thumb     string
	title     string
	performer string
}

// parseAjaxURL extracts the inner content URL the outer page would
// AJAX-load. Returns ("", false) if the outer doesn't have one (the
// site uses a different template entirely and isn't covered).
func parseAjaxURL(body []byte) (string, bool) {
	m := outerAjaxRe.FindSubmatch(body)
	if m == nil {
		return "", false
	}
	return string(m[1]), true
}

// parseMaxOffset returns the highest `p={N}` referenced on the outer
// page. Returns 0 if no pagination links are present (single-page
// catalogue or the AJAX hash didn't expose them).
func parseMaxOffset(body []byte) int {
	maxPage := 0
	for _, m := range outerMaxPageRe.FindAllSubmatch(body, -1) {
		if n, _ := strconv.Atoi(string(m[1])); n > maxPage {
			maxPage = n
		}
	}
	return maxPage
}

// parseInnerCards extracts every `img-portfolio` card from the inner
// AJAX-loaded list.
func parseInnerCards(body []byte) []card {
	page := string(body)
	starts := innerCardStartRe.FindAllStringIndex(page, -1)
	cards := make([]card, 0, len(starts))
	seen := make(map[string]bool, len(starts))

	for i, loc := range starts {
		end := len(page)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		block := page[loc[1]:end]

		var c card
		if m := innerSceneIDRe.FindStringSubmatch(block); m != nil {
			c.id = m[1]
		}
		if c.id == "" || seen[c.id] {
			continue
		}
		seen[c.id] = true

		if m := innerThumbRe.FindStringSubmatch(block); m != nil {
			c.thumb = m[1]
		}
		if m := innerTitleRe.FindStringSubmatch(block); m != nil {
			c.title = cleanText(m[1])
		}
		if m := innerPerformerRe.FindStringSubmatch(block); m != nil {
			c.performer = cleanText(m[1])
		}

		cards = append(cards, c)
	}
	return cards
}

var wsRe = regexp.MustCompile(`\s+`)

func cleanText(s string) string {
	if s == "" {
		return ""
	}
	s = html.UnescapeString(s)
	s = wsRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// ---- run loop ----

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)
	scraper.Debugf(1, "feetondemand/%s: scraping Videos catalogue", s.cfg.ID)

	now := time.Now().UTC()
	sentTotal := false
	emitted := 0

	for offset := 0; ; offset += cardsPerPage {
		if ctx.Err() != nil {
			return
		}
		if offset > 0 && opts.Delay > 0 {
			cancelled := false
			select {
			case <-time.After(opts.Delay):
			case <-ctx.Done():
				cancelled = true
			}
			if cancelled {
				return
			}
		}

		scraper.Debugf(1, "feetondemand/%s: fetching offset %d", s.cfg.ID, offset)
		outer, err := s.fetchOuter(ctx, offset)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("outer offset=%d: %w", offset, err)):
			case <-ctx.Done():
			}
			return
		}

		// First page: derive the total scene count from the highest
		// `p=N` reference, send Progress before walking.
		if !sentTotal {
			maxOff := parseMaxOffset(outer)
			if maxOff > 0 {
				// Pages run 0, 20, 40, …, maxOff inclusive →
				// (maxOff/cardsPerPage + 1) pages × cardsPerPage cards
				// (last page may have fewer; this is an estimate).
				total := (maxOff/cardsPerPage + 1) * cardsPerPage
				scraper.Debugf(1, "feetondemand/%s: ~%d total scenes (last offset %d)", s.cfg.ID, total, maxOff)
				select {
				case out <- scraper.Progress(total):
				case <-ctx.Done():
					return
				}
			}
			sentTotal = true
		}

		ajaxPath, ok := parseAjaxURL(outer)
		if !ok {
			scraper.Debugf(1, "feetondemand/%s: no AJAX URL on outer page — site template not supported", s.cfg.ID)
			return
		}

		inner, err := s.fetchInner(ctx, ajaxPath)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("inner offset=%d: %w", offset, err)):
			case <-ctx.Done():
			}
			return
		}

		cards := parseInnerCards(inner)
		if len(cards) == 0 {
			// Past the end of the catalogue.
			scraper.Debugf(1, "feetondemand/%s: catalogue exhausted at offset %d (%d scenes emitted)", s.cfg.ID, offset, emitted)
			return
		}

		for _, c := range cards {
			if opts.KnownIDs[c.id] {
				scraper.Debugf(1, "feetondemand/%s: hit known ID %s, stopping early", s.cfg.ID, c.id)
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
				return
			}
			select {
			case out <- scraper.Scene(s.toScene(c, studioURL, now)):
				emitted++
			case <-ctx.Done():
				return
			}
		}
	}
}

func (s *Scraper) fetchOuter(ctx context.Context, offset int) ([]byte, error) {
	u := fmt.Sprintf("%s/?mb=%s&p=%d", s.cfg.BaseURL, videosMb, offset)
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     u,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}

func (s *Scraper) fetchInner(ctx context.Context, path string) ([]byte, error) {
	// path is relative — e.g. `content/pages/ffzpzgvvc3x8fhw….list.htm`.
	u := s.cfg.BaseURL + "/" + strings.TrimPrefix(path, "/")
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     u,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}

func (s *Scraper) toScene(c card, studioURL string, now time.Time) models.Scene {
	scene := models.Scene{
		ID:        c.id,
		SiteID:    s.cfg.ID,
		StudioURL: studioURL,
		Title:     c.title,
		// No public detail page — scenes play in a Bootstrap modal on
		// the same page. Synthesise a stable anchor URL so each scene
		// has something downstream tools can deep-link to.
		URL:       fmt.Sprintf("%s/?mb=%s#pop_%s", s.cfg.BaseURL, videosMb, c.id),
		Thumbnail: c.thumb,
		Studio:    studioName,
		Series:    s.cfg.SiteName,
		ScrapedAt: now,
	}
	if c.performer != "" {
		scene.Performers = []string{c.performer}
	}
	return scene
}
