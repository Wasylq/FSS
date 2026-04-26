package transqueens

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/sexmexutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *sexmexutil.Scraper {
	return sexmexutil.New(sexmexutil.SiteConfig{
		ID:       "transqueens",
		Studio:   "Trans Queens",
		SiteBase: "https://transqueens.com",
		Patterns: []string{
			"transqueens.com",
			"transqueens.com/tour/updates",
			"transqueens.com/tour/models/{slug}.html",
			"transqueens.com/tour/categories/{slug}.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?transqueens\.com(/|$)`),
	})
}

func init() { scraper.Register(New()) }
