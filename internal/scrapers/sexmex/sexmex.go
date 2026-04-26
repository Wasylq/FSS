package sexmex

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/sexmexutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *sexmexutil.Scraper {
	return sexmexutil.New(sexmexutil.SiteConfig{
		ID:       "sexmex",
		Studio:   "SexMex",
		SiteBase: "https://sexmex.xxx",
		Patterns: []string{
			"sexmex.xxx",
			"sexmex.xxx/tour/updates",
			"sexmex.xxx/tour/models/{slug}.html",
			"sexmex.xxx/tour/categories/{slug}.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?sexmex\.xxx(/|$)`),
	})
}

func init() { scraper.Register(New()) }
