package littlefromasia

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/veutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *veutil.Scraper {
	return veutil.New(veutil.SiteConfig{
		ID:             "littlefromasia",
		Studio:         "LittleFromAsia",
		SiteBase:       "https://littlefromasia.com",
		MainCategoryID: 1,
		Patterns:       []string{"littlefromasia.com"},
		MatchRe:        regexp.MustCompile(`^https?://(?:www\.)?littlefromasia\.com(/|$)`),
	})
}

func init() { scraper.Register(New()) }
