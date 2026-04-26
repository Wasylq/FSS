package exposedlatinas

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/sexmexutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *sexmexutil.Scraper {
	return sexmexutil.New(sexmexutil.SiteConfig{
		ID:       "exposedlatinas",
		Studio:   "Exposed Latinas",
		SiteBase: "https://exposedlatinas.com",
		Patterns: []string{
			"exposedlatinas.com",
			"exposedlatinas.com/tour/updates",
			"exposedlatinas.com/tour/models/{slug}.html",
			"exposedlatinas.com/tour/categories/{slug}.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?exposedlatinas\.com(/|$)`),
	})
}

func init() { scraper.Register(New()) }
