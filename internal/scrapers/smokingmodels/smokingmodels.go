package smokingmodels

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/railwayutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *railwayutil.Scraper {
	return railwayutil.New(railwayutil.SiteConfig{
		ID:       "smokingmodels",
		SiteCode: "SM",
		Studio:   "Smoking Models",
		SiteBase: "https://smokingmodels.com",
		Patterns: []string{
			"smokingmodels.com/#/models",
			"smokingmodels.com/#/models/{name}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?smokingmodels\.com`),
	})
}

func init() { scraper.Register(New()) }
