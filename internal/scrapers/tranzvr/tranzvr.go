package tranzvr

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/povrutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *povrutil.Scraper {
	return povrutil.New(povrutil.SiteConfig{
		ID:       "tranzvr",
		Studio:   "TranzVR",
		SiteBase: "https://www.tranzvr.com",
		Patterns: []string{
			"tranzvr.com",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?tranzvr\.com(/|$)`),
	})
}

func init() { scraper.Register(New()) }
