package mypervmom

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/veutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *veutil.Scraper {
	return veutil.New(veutil.SiteConfig{
		ID:             "mypervmom",
		Studio:         "PervMom",
		SiteBase:       "https://mypervmom.com",
		MainCategoryID: 1,
		Patterns:       []string{"mypervmom.com"},
		MatchRe:        regexp.MustCompile(`^https?://(?:www\.)?mypervmom\.com(/|$)`),
	})
}

func init() { scraper.Register(New()) }
