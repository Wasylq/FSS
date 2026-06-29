// Package coedproductions registers the Coed Productions network sites (NATS
// "updateItem" tour template). See coedutil.
package coedproductions

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/coedutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []coedutil.SiteConfig{
	{
		ID:       "nebraskacoeds",
		Studio:   "Nebraska Coeds",
		SiteBase: "https://tour.nebraskacoeds.com",
		Patterns: []string{"nebraskacoeds.com", "tour.nebraskacoeds.com/categories/updates_{N}_d.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.|tour\.)?nebraskacoeds\.com`),
	},
	{
		ID:       "misspussycat",
		Studio:   "Miss Pussycat",
		SiteBase: "https://misspussycat.com",
		Patterns: []string{"misspussycat.com", "misspussycat.com/categories/updates_{N}_d.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?misspussycat\.com`),
	},
	{
		ID:       "afterhoursexposed",
		Studio:   "After Hours Exposed",
		SiteBase: "https://afterhoursexposed.com",
		Patterns: []string{"afterhoursexposed.com", "afterhoursexposed.com/categories/updates_{N}_d.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?afterhoursexposed\.com`),
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(coedutil.New(cfg))
	}
}
