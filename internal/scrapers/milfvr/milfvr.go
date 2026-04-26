package milfvr

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/povrutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *povrutil.Scraper {
	return povrutil.New(povrutil.SiteConfig{
		ID:       "milfvr",
		Studio:   "MilfVR",
		SiteBase: "https://www.milfvr.com",
		Patterns: []string{
			"milfvr.com",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?milfvr\.com(/|$)`),
	})
}

func init() { scraper.Register(New()) }
