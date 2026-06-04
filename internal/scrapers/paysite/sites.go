package paysite

import (
	"regexp"

	"github.com/Wasylq/FSS/scraper"
)

var sites = []SiteConfig{
	{
		ID:       "ladysonia",
		SiteBase: "https://tour.lady-sonia.com",
		Patterns: []string{"lady-sonia.com"},
		MatchRe:  regexp.MustCompile(`^https?://(?:(?:www|tour)\.)?lady-sonia\.com(?:/|$)`),
	},
	{
		ID:       "mariskax",
		SiteBase: "https://tour.mariskax.com",
		Patterns: []string{"mariskax.com"},
		MatchRe:  regexp.MustCompile(`^https?://(?:(?:www|tour)\.)?mariskax\.com(?:/|$)`),
	},
	{
		ID:       "inkedpov",
		SiteBase: "https://inkedpov.com",
		Patterns: []string{"inkedpov.com"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?inkedpov\.com(?:/|$)`),
	},
	{
		ID:       "katerinahartlova",
		SiteBase: "https://tour.katerina-hartlova.com",
		Patterns: []string{"katerina-hartlova.com"},
		MatchRe:  regexp.MustCompile(`^https?://(?:(?:www|tour)\.)?katerina-hartlova\.com(?:/|$)`),
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(New(cfg))
	}
}
