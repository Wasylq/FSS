package brattyfamily

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/veutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *veutil.Scraper {
	return veutil.New(veutil.SiteConfig{
		ID:             "brattyfamily",
		Studio:         "BrattyFamily",
		SiteBase:       "https://brattyfamily.com",
		MainCategoryID: 1,
		Patterns:       []string{"brattyfamily.com"},
		MatchRe:        regexp.MustCompile(`^https?://(?:www\.)?brattyfamily\.com(/|$)`),
	})
}

func init() { scraper.Register(New()) }
