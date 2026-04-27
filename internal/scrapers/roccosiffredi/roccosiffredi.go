package roccosiffredi

import (
	"context"
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/gammautil"
	"github.com/Wasylq/FSS/scraper"
)

var config = gammautil.SiteConfig{
	SiteID:     "roccosiffredi",
	SiteBase:   "https://www.roccosiffredi.com",
	StudioName: "Rocco Siffredi",
	SiteName:   "roccosiffredi",
}

type Scraper struct {
	gamma *gammautil.Scraper
}

func New() *Scraper {
	return &Scraper{gamma: gammautil.NewScraper(config)}
}

func init() {
	scraper.Register(New())
}

func (s *Scraper) ID() string { return config.SiteID }

func (s *Scraper) Patterns() []string {
	return []string{"roccosiffredi.com"}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?roccosiffredi\.com`)

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.gamma.Run(ctx, studioURL, opts, out)
	return out, nil
}
