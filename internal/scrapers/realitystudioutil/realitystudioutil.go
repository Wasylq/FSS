// Package realitystudioutil scrapes the Reality Studio LLC fetish sites
// (Subby Girls, Female Worship, Men Are Slaves, Cum Countdown). Every site is
// the same hand-rolled JS tour whose entire catalog is a static JavaScript
// array in /js/clips.js — no pagination, no API, no auth. Each row is:
//
//	[ filename, title, date, pictureCount, performers, fetishes, flag ]
//
// The per-scene gallery page is /gallery.html?{index} and the poster is
// /images/gallery/{filename}/Pics/{filename}1.jpg.
package realitystudioutil

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
	SiteBase string // e.g. "https://www.subbygirls.com" — no trailing slash
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

// rowRe captures the 7 quoted fields of one ClipDatabase entry. Fields never
// contain embedded double quotes; whitespace around commas varies per row.
var rowRe = regexp.MustCompile(`\[\s*"([^"]*)"\s*,\s*"([^"]*)"\s*,\s*"([^"]*)"\s*,\s*"([^"]*)"\s*,\s*"([^"]*)"\s*,\s*"([^"]*)"\s*,\s*"([^"]*)"\s*\]`)

var dateLayouts = []string{"01-02-06", "01/02/06", "01-02-2006", "01/02/2006"}

func (s *Scraper) run(ctx context.Context, studioURL string, _ scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	scraper.Debugf(1, "%s: fetching clips.js", s.cfg.ID)
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL:     s.cfg.SiteBase + "/js/clips.js",
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		s.send(ctx, out, scraper.Error(fmt.Errorf("fetching clips.js: %w", err)))
		return
	}
	body, err := func() ([]byte, error) {
		defer func() { _ = resp.Body.Close() }()
		return httpx.ReadBody(resp.Body)
	}()
	if err != nil {
		s.send(ctx, out, scraper.Error(fmt.Errorf("reading clips.js: %w", err)))
		return
	}

	rows := rowRe.FindAllStringSubmatch(string(body), -1)
	now := time.Now().UTC()

	// Count real scenes (skip empty rows) for the progress total.
	total := 0
	for _, r := range rows {
		if r[1] != "" {
			total++
		}
	}
	scraper.Debugf(1, "%s: parsed %d clip rows (%d non-empty)", s.cfg.ID, len(rows), total)
	s.send(ctx, out, scraper.Progress(total))

	for idx, r := range rows {
		filename := r[1]
		if filename == "" {
			continue // header / placeholder row
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
		s.send(ctx, out, scraper.Scene(s.toScene(studioURL, idx, r, now)))
	}
}

func (s *Scraper) toScene(studioURL string, idx int, r []string, now time.Time) models.Scene {
	filename := r[1]
	title := html.UnescapeString(strings.TrimSpace(r[2]))
	if title == "" {
		title = filename
	}

	var date time.Time
	if d := strings.TrimSpace(r[3]); d != "" {
		date, _ = parseutil.TryParseDate(d, dateLayouts...)
	}

	performers := splitList(r[5])
	tags := splitList(r[6])

	return models.Scene{
		ID:         filename,
		SiteID:     s.cfg.ID,
		StudioURL:  studioURL,
		Title:      title,
		URL:        fmt.Sprintf("%s/gallery.html?%d", s.cfg.SiteBase, idx),
		Date:       date,
		Thumbnail:  fmt.Sprintf("%s/images/gallery/%s/Pics/%s1.jpg", s.cfg.SiteBase, filename, filename),
		Performers: performers,
		Tags:       tags,
		Studio:     s.cfg.Studio,
		ScrapedAt:  now,
	}
}

func splitList(s string) []string {
	s = html.UnescapeString(strings.TrimSpace(s))
	if s == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func (s *Scraper) send(ctx context.Context, out chan<- scraper.SceneResult, r scraper.SceneResult) {
	select {
	case out <- r:
	case <-ctx.Done():
	}
}
