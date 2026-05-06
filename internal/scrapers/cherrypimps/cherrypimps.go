package cherrypimps

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/cherrypimpsutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []cherrypimpsutil.SiteConfig{
	{
		ID:       "cherrypimps",
		SiteBase: "https://cherrypimps.com",
		Studio:   "Cherry Pimps",
		Patterns: []string{
			"cherrypimps.com",
			"cherrypimps.com/series/{slug}",
			"cherrypimps.com/categories/{slug}",
			"cherrypimps.com/models/{name}.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?cherrypimps\.com`),
	},
	{
		ID:       "wildoncam",
		SiteBase: "https://wildoncam.com",
		Studio:   "Wild on Cam",
		Patterns: []string{
			"wildoncam.com",
			"wildoncam.com/series/{slug}",
			"wildoncam.com/categories/{slug}",
			"wildoncam.com/models/{name}.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?wildoncam\.com`),
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(cherrypimpsutil.New(cfg))
	}
}
