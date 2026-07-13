// Package rubberpassion scrapes Rubber Passion (rubber-passion.com), a latex
// fetish site on the MyMember.site platform. All listing and detail handling
// lives in mymemberutil; this package only supplies the site config.
package rubberpassion

import (
	"context"
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/mymemberutil"
	"github.com/Wasylq/FSS/scraper"
)

// knownPerformers splits the detail page's comma-joined keyword list into
// performers and tags. Derived from a sweep of the full ~525-scene catalogue;
// everything not listed here (Catsuit, Bondage, Medical, …) becomes a tag.
// Several entries are personas of the same model (Latex Lucy / Retro Lucy /
// Fetish Rose) — they are kept distinct because that is how the site credits
// them.
var mm = mymemberutil.New(mymemberutil.SiteConfig{
	SiteID:     "rubberpassion",
	Domain:     "rubber-passion.com",
	StudioName: "Rubber Passion",
	KnownPerformers: map[string]bool{
		"latex lucy":      true,
		"retro lucy":      true,
		"fetish rose":     true,
		"rubber rhylee":   true,
		"mistress lovisa": true,
		"rebecca smyth":   true,
		"zara du rose":    true,
		"lola noir":       true,
		"brook maddison":  true,
		"mike majors":     true,
	},
})

// Scraper implements scraper.StudioScraper for Rubber Passion.
type Scraper struct {
	mm *mymemberutil.Scraper
}

// New constructs a Rubber Passion scraper.
func New() *Scraper {
	return &Scraper{mm: mm}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() {
	scraper.Register(New())
}

func (s *Scraper) ID() string         { return mm.Config().SiteID }
func (s *Scraper) Patterns() []string { return []string{"rubber-passion.com"} }

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?rubber-passion\.com(?:/|$)`)

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go func() {
		defer close(out)
		s.mm.Run(ctx, studioURL, opts, out)
	}()
	return out, nil
}
