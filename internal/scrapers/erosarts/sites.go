package erosarts

import (
	"regexp"

	"github.com/Wasylq/FSS/scraper"
)

var sites = []SiteConfig{
	{
		ID:       "jerkoffinstructions",
		SiteBase: "https://jerkoffinstructions.com",
		SiteName: "Jerk Off Instructions",
		Patterns: []string{"jerkoffinstructions.com"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?jerkoffinstructions\.com(?:/|$)`),
	},
	{
		ID:       "sexpov",
		SiteBase: "https://sexpov.com",
		SiteName: "SexPOV",
		Patterns: []string{"sexpov.com"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?sexpov\.com(?:/|$)`),
	},
	{
		ID:       "stepmomfun",
		SiteBase: "https://stepmomfun.com",
		SiteName: "Stepmom Fun",
		Patterns: []string{"stepmomfun.com"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?stepmomfun\.com(?:/|$)`),
	},
	{
		ID:       "taboohandjobs",
		SiteBase: "https://taboohandjobs.com",
		SiteName: "Taboo Handjobs",
		Patterns: []string{"taboohandjobs.com"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?taboohandjobs\.com(?:/|$)`),
	},
	{
		ID:       "taboopov",
		SiteBase: "https://taboopov.com",
		SiteName: "Taboo POV",
		Patterns: []string{"taboopov.com"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?taboopov\.com(?:/|$)`),
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(New(cfg))
	}
}
