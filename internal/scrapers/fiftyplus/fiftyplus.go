package fiftyplus

import (
	"context"
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/scoregrouputil"
	"github.com/Wasylq/FSS/scraper"
)

var config = scoregrouputil.SiteConfig{
	SiteID:     "50plusmilfs",
	SiteBase:   "https://www.50plusmilfs.com",
	StudioName: "50 Plus MILFs",
	VideosPath: "/xxx-milf-videos/",
	ModelsPath: "/xxx-milf-models/",
}

type Scraper struct {
	sg *scoregrouputil.Scraper
}

func New() *Scraper {
	return &Scraper{sg: scoregrouputil.NewScraper(config)}
}

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return config.SiteID }

func (s *Scraper) Patterns() []string {
	return []string{
		"50plusmilfs.com",
		"50plusmilfs.com/xxx-milf-models/{name}/{id}",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?50plusmilfs\.com`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.sg.Run(ctx, studioURL, opts, out)
	return out, nil
}
