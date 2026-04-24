package digitalplayground

import (
	"context"
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/ayloutil"
	"github.com/Wasylq/FSS/scraper"
)

var config = ayloutil.SiteConfig{
	SiteID:     "digitalplayground",
	SiteBase:   "https://www.digitalplayground.com",
	StudioName: "Digital Playground",
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
		"digitalplayground.com",
		"digitalplayground.com/pornstar/{id}/{slug}",
		"digitalplayground.com/category/{id}/{slug}",
		"digitalplayground.com/site/{id}/{slug}",
		"digitalplayground.com/series/{id}/{slug}",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?digitalplayground\.com`)

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.aylo.Run(ctx, studioURL, opts, out)
	return out, nil
}
