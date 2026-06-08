package kingnoirexxx

import (
	"context"
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/mymemberutil"
	"github.com/Wasylq/FSS/scraper"
)

var mm = mymemberutil.New(mymemberutil.SiteConfig{
	SiteID:     "kingnoirexxx",
	Domain:     "kingnoirexxx.com",
	StudioName: "KingNoireXXX",
	KnownPerformers: map[string]bool{
		"king noire":          true,
		"jet setting jasmine": true,
		"freckle lemonade":    true,
		"billie beaumont":     true,
		"lena paul":           true,
		"mocha menage":        true,
		"carmela clutch":      true,
		"fae laveau":          true,
		"jupiter jetson":      true,
		"leah michelle":       true,
		"jenna love":          true,
		"amarilla diosa":      true,
		"danja angel":         true,
		"ravyn alexa":         true,
		"ivy brooks":          true,
		"bishop black":        true,
		"charlie chaste":      true,
	},
})

type Scraper struct {
	mm *mymemberutil.Scraper
}

func New() *Scraper {
	return &Scraper{mm: mm}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() {
	scraper.Register(New())
}

func (s *Scraper) ID() string         { return mm.Config().SiteID }
func (s *Scraper) Patterns() []string { return []string{"kingnoirexxx.com"} }

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?kingnoirexxx\.com(?:/|$)`)

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
