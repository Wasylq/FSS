package wankzvr

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/povrutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *povrutil.Scraper {
	return povrutil.New(povrutil.SiteConfig{
		ID:       "wankzvr",
		Studio:   "WankzVR",
		SiteBase: "https://www.wankzvr.com",
		Patterns: []string{
			"wankzvr.com",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?wankzvr\.com(/|$)`),
	})
}

func init() { scraper.Register(New()) }
