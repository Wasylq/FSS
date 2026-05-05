package spicevids

import (
	"context"
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/ayloutil"
	"github.com/Wasylq/FSS/scraper"
)

var ayloConfig = ayloutil.SiteConfig{
	SiteID:     "spicevids",
	SiteBase:   "https://www.spicevids.com",
	StudioName: "SpiceVids",
	ScenePath:  "scene",
}

func init() {
	scraper.Register(&spicevidsScraper{aylo: ayloutil.NewScraper(ayloConfig)})
}

type spicevidsScraper struct {
	aylo *ayloutil.Scraper
}

func (s *spicevidsScraper) ID() string { return "spicevids" }

func (s *spicevidsScraper) Patterns() []string {
	return []string{
		"spicevids.com",
		"spicevids.com/model/{id}/{slug}",
		"spicevids.com/collection/{id}/{slug}",
		"spicevids.com/category/{id}/{slug}",
		"spicevids.com/series/{id}/{slug}",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?spicevids\.com`)

func (s *spicevidsScraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *spicevidsScraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.aylo.Run(ctx, studioURL, opts, out)
	return out, nil
}
