package mydaughterswap

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/veutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *veutil.Scraper {
	return veutil.New(veutil.SiteConfig{
		ID:             "mydaughterswap",
		Studio:         "DaughterSwap",
		SiteBase:       "https://mydaughterswap.com",
		MainCategoryID: 1,
		Patterns:       []string{"mydaughterswap.com"},
		MatchRe:        regexp.MustCompile(`^https?://(?:www\.)?mydaughterswap\.com(/|$)`),
	})
}

func init() { scraper.Register(New()) }
