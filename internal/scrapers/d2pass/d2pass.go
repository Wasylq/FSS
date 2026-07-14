// Package d2pass registers the D2Pass "bifrost" sites (1Pondo, 10musume,
// Pacopacomama, Muramura). All listing and scene handling lives in
// d2passutil; this package only supplies the site table.
//
// Caribbeancom, Caribbeancompr and Heyzo are commonly grouped with these but
// are NOT on this platform — their /dyn/phpauto/ paths 404, so they need
// separate scrapers.
package d2pass

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Wasylq/FSS/internal/scrapers/d2passutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []d2passutil.SiteConfig{
	{SiteID: "1pondo", Domain: "www.1pondo.tv", StudioName: "1Pondo"},
	{SiteID: "10musume", Domain: "www.10musume.com", StudioName: "10musume"},
	{SiteID: "pacopacomama", Domain: "www.pacopacomama.com", StudioName: "Pacopacomama"},
	{SiteID: "muramura", Domain: "www.muramura.tv", StudioName: "Muramura"},
}

// Scraper adapts a d2passutil instance to the StudioScraper interface.
type Scraper struct {
	d2      *d2passutil.Scraper
	matchRe *regexp.Regexp
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func newScraper(cfg d2passutil.SiteConfig) *Scraper {
	// Match the bare domain with or without the www. prefix, since the site
	// serves identical content on both (en.1pondo.tv included).
	bare := strings.TrimPrefix(cfg.Domain, "www.")
	escaped := strings.ReplaceAll(bare, ".", `\.`)
	return &Scraper{
		d2:      d2passutil.New(cfg),
		matchRe: regexp.MustCompile(fmt.Sprintf(`^https?://(?:[a-z0-9-]+\.)?%s(?:/|$)`, escaped)),
	}
}

func init() {
	for _, cfg := range sites {
		scraper.Register(newScraper(cfg))
	}
}

func (s *Scraper) ID() string { return s.d2.Config().SiteID }

func (s *Scraper) Patterns() []string {
	d := s.d2.Config().Domain
	return []string{d, d + "/movies/{MovieID}/"}
}

func (s *Scraper) MatchesURL(u string) bool { return s.matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go func() {
		defer close(out)
		s.d2.Run(ctx, studioURL, opts, out)
	}()
	return out, nil
}
