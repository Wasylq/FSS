package rachelsteele

import (
	"context"
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/mymemberutil"
	"github.com/Wasylq/FSS/scraper"
)

var mm = mymemberutil.New(mymemberutil.SiteConfig{
	SiteID:     "rachelsteele",
	Domain:     "rachel-steele.com",
	StudioName: "Rachel Steele",
	KnownPerformers: map[string]bool{
		"ophelia fae":       true,
		"lily starfire":     true,
		"reya lovenlight":   true,
		"mia simone":        true,
		"mindi mink":        true,
		"leo malonee":       true,
		"pixie smalls":      true,
		"cherie deville":    true,
		"danni jones":       true,
		"ryan keely":        true,
		"josh rivers":       true,
		"anthony pierce":    true,
		"karen fisher":      true,
		"mellanie monroe":   true,
		"richard glaze":     true,
		"damson jenkins":    true,
		"rachael cavalli":   true,
		"london river":      true,
		"max fills":         true,
		"hailey rose":       true,
		"slave marcelo":     true,
		"tyler cruise":      true,
		"honey heston":      true,
		"ares":              true,
		"dallas diamondz":   true,
		"kenny koxx":        true,
		"brycen ward":       true,
		"leihla":            true,
		"keri lynn":         true,
		"arianna labarbara": true,
		"rachel steele":     true,
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
func (s *Scraper) Patterns() []string { return []string{"rachel-steele.com"} }

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?rachel-steele\.com(?:/|$)`)

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
