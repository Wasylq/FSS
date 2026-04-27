package twistys

import (
	"context"
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/ayloutil"
	"github.com/Wasylq/FSS/scraper"
)

var config = ayloutil.SiteConfig{
	SiteID:     "twistys",
	SiteBase:   "https://www.twistys.com",
	StudioName: "Twistys",
}

type Scraper struct {
	aylo *ayloutil.Scraper
}

func New() *Scraper {
	return &Scraper{aylo: ayloutil.NewScraper(config)}
}

func init() {
	scraper.Register(New())
}

func (s *Scraper) ID() string { return config.SiteID }

func (s *Scraper) Patterns() []string {
	return []string{
		"twistys.com",
		"twistys.com/pornstar/{id}/{slug}",
		"twistys.com/category/{id}/{slug}",
		"twistys.com/site/{id}/{slug}",
		"twistys.com/series/{id}/{slug}",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?twistys\.com`)

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.aylo.Run(ctx, studioURL, opts, out)
	return out, nil
}
