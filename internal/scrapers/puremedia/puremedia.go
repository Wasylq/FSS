// Package puremedia registers the Pure Media Enterprises trans/BBW network
// sites. All six share one static tour CMS; see puremediautil.
package puremedia

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/puremediautil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []puremediautil.SiteConfig{
	{
		ID:       "purets",
		Studio:   "PureTS",
		SiteBase: "https://pure-ts.com",
		Patterns: []string{"pure-ts.com", "pure-ts.com/tour/models/{Model}.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?pure-ts\.com`),
	},
	{
		ID:       "purebbw",
		Studio:   "Pure BBW",
		SiteBase: "https://pure-bbw.com",
		Patterns: []string{"pure-bbw.com", "pure-bbw.com/tour/models/{Model}.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?pure-bbw\.com`),
	},
	{
		ID:       "tspov",
		Studio:   "TSPOV",
		SiteBase: "https://tspov.com",
		Patterns: []string{"tspov.com", "tspov.com/tour/models/{Model}.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?tspov\.com`),
	},
	{
		ID:       "purexxx",
		Studio:   "PureXXX",
		SiteBase: "https://pure-xxx.com",
		Patterns: []string{"pure-xxx.com", "pure-xxx.com/tour/models/{Model}.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?pure-xxx\.com`),
	},
	{
		ID:       "becomingfemme",
		Studio:   "Becoming Femme",
		SiteBase: "https://becomingfemme.com",
		Patterns: []string{"becomingfemme.com", "becomingfemme.com/tour/models/{Model}.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?becomingfemme\.com`),
	},
	{
		ID:       "sissypov",
		Studio:   "Sissy POV",
		SiteBase: "https://sissypov.com",
		Patterns: []string{"sissypov.com", "sissypov.com/tour/models/{Model}.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?sissypov\.com`),
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(puremediautil.New(cfg))
	}
}
