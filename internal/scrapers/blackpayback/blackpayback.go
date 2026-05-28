// Package blackpayback registers the Black Payback scraper — the lone
// Elevated X "Classic" theme site in the Adult Doorway tree. All parser
// logic lives in adultdoorwayclassicutil; this file is the site config.
package blackpayback

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/adultdoorwayclassicutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []adultdoorwayclassicutil.SiteConfig{
	{
		ID:       "blackpayback",
		SiteBase: "https://blackpayback.com",
		Studio:   "Black Payback",
		Patterns: []string{
			"blackpayback.com",
			"blackpayback.com/tour/categories/movies/{page}/latest/",
			"blackpayback.com/tour/categories/{slug}/{page}/latest/",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:[a-z0-9-]+\.)?blackpayback\.com`),
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(adultdoorwayclassicutil.New(cfg))
	}
}
