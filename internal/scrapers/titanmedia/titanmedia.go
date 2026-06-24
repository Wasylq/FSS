// Package titanmedia registers the Titan Media "gloryhole" network sites. See
// titanmediautil. GloryholeSwallow and CumClinic serve their tour under /tour;
// Cumpsters and SpyTug serve it at the site root.
package titanmedia

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/titanmediautil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []titanmediautil.SiteConfig{
	{
		ID:       "gloryholeswallow",
		Studio:   "Gloryhole Swallow",
		SiteBase: "https://gloryholeswallow.com/tour",
		Patterns: []string{"gloryholeswallow.com", "gloryholeswallow.com/tour/categories/Movies_{N}_d.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?gloryholeswallow\.com`),
	},
	{
		ID:       "cumclinic",
		Studio:   "CumClinic",
		SiteBase: "https://cumclinic.com/tour",
		Patterns: []string{"cumclinic.com", "cumclinic.com/tour/categories/Movies_{N}_d.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?cumclinic\.com`),
	},
	{
		ID:       "cumpsters",
		Studio:   "Cumpsters",
		SiteBase: "https://cumpsters.com",
		Patterns: []string{"cumpsters.com", "cumpsters.com/categories/Movies_{N}_d.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?cumpsters\.com`),
	},
	{
		ID:       "spytug",
		Studio:   "SpyTug",
		SiteBase: "https://spytug.com",
		Patterns: []string{"spytug.com", "spytug.com/categories/Movies_{N}_d.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?spytug\.com`),
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(titanmediautil.New(cfg))
	}
}
