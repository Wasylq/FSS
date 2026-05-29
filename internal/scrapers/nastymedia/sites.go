package nastymedia

import (
	"regexp"

	"github.com/Wasylq/FSS/scraper"
)

// sites lists the 4 live Nasty Media Group sites. The other 9 stashdb
// children are DNS-dead, parked, or domain-squatted — see the package
// doc for details.
var sites = []SiteConfig{
	{
		ID:       "coozhound",
		BaseURL:  "https://www.coozhound.com",
		SiteName: "CoozHound",
		Patterns: []string{"https://www.coozhound.com/HOME.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?coozhound\.com\b`),
	},
	{
		ID:       "msnympho",
		BaseURL:  "https://www.msnympho.com",
		SiteName: "MsNympho",
		Patterns: []string{"https://www.msnympho.com/HOME.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?msnympho\.com\b`),
	},
	{
		ID:       "nastynyamateurs",
		BaseURL:  "https://www.nastynyamateurs.com",
		SiteName: "Nasty NY Amateurs",
		Patterns: []string{"https://nastynyamateurs.com/HOME.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?nastynyamateurs\.com\b`),
	},
	{
		ID:       "urbanamateurs",
		BaseURL:  "https://www.urbanamateurs.net",
		SiteName: "Urban Amateurs",
		Patterns: []string{"https://www.urbanamateurs.net/HOME.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?urbanamateurs\.net\b`),
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(New(cfg))
	}
}
