package youngerloverofmine

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/veutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *veutil.Scraper {
	return veutil.New(veutil.SiteConfig{
		ID:             "youngerloverofmine",
		Studio:         "YoungerLoverOfMine",
		SiteBase:       "https://youngerloverofmine.com",
		MainCategoryID: 1,
		Patterns:       []string{"youngerloverofmine.com"},
		MatchRe:        regexp.MustCompile(`^https?://(?:www\.)?youngerloverofmine\.com(/|$)`),
	})
}

func init() { scraper.Register(New()) }
