package smokingerotica

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/railwayutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *railwayutil.Scraper {
	return railwayutil.New(railwayutil.SiteConfig{
		ID:       "smokingerotica",
		SiteCode: "SE",
		Studio:   "Smoking Erotica",
		SiteBase: "https://smokingerotica.com",
		Patterns: []string{
			"smokingerotica.com/#/models",
			"smokingerotica.com/#/models/{name}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?smokingerotica\.com`),
	})
}

func init() { scraper.Register(New()) }
