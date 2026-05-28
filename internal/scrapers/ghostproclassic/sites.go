package ghostproclassic

import (
	"regexp"

	"github.com/Wasylq/FSS/scraper"
)

// sites lists the four Ghost Pro Productions sister sites running the
// Elevated-X-Classic HTML template. Each registers its own scraper instance.
var sites = []SiteConfig{
	{
		ID:       "analjesse",
		SiteBase: "https://analjesse.com",
		SiteName: "Anal Jesse",
		Patterns: []string{"analjesse.com/", "analjesse.com/categories/updates_{N}_p.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?analjesse\.com`),
	},
	{
		ID:       "creampiethais",
		SiteBase: "https://creampiethais.com",
		SiteName: "Creampie Thais",
		Patterns: []string{"creampiethais.com/", "creampiethais.com/categories/updates_{N}_p.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?creampiethais\.com`),
	},
	{
		ID:       "mongerinasia",
		SiteBase: "https://mongerinasia.com",
		SiteName: "Monger in Asia",
		Patterns: []string{"mongerinasia.com/", "mongerinasia.com/categories/updates_{N}_p.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?mongerinasia\.com`),
	},
	{
		ID:       "tailynn",
		SiteBase: "https://tailynn.com",
		SiteName: "Tailynn",
		Patterns: []string{"tailynn.com/", "tailynn.com/categories/updates_{N}_p.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?tailynn\.com`),
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(New(cfg))
	}
}
