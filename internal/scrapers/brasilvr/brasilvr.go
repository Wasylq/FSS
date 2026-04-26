package brasilvr

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/povrutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *povrutil.Scraper {
	return povrutil.New(povrutil.SiteConfig{
		ID:       "brasilvr",
		Studio:   "BrasilVR",
		SiteBase: "https://www.brasilvr.com",
		Patterns: []string{
			"brasilvr.com",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?brasilvr\.com(/|$)`),
	})
}

func init() { scraper.Register(New()) }
