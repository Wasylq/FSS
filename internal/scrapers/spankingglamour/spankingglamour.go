package spankingglamour

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/railwayutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *railwayutil.Scraper {
	return railwayutil.New(railwayutil.SiteConfig{
		ID:       "spankingglamour",
		SiteCode: "SPG",
		Studio:   "Spanking Glamour",
		SiteBase: "https://spankingglamour.com",
		Patterns: []string{
			"spankingglamour.com/#/models",
			"spankingglamour.com/#/models/{name}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?spankingglamour\.com`),
	})
}

func init() { scraper.Register(New()) }
