package mysislovesme

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/veutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *veutil.Scraper {
	return veutil.New(veutil.SiteConfig{
		ID:             "mysislovesme",
		Studio:         "SisLovesMe",
		SiteBase:       "https://mysislovesme.com",
		MainCategoryID: 1,
		Patterns:       []string{"mysislovesme.com"},
		MatchRe:        regexp.MustCompile(`^https?://(?:www\.)?mysislovesme\.com(/|$)`),
	})
}

func init() { scraper.Register(New()) }
