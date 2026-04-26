package hugecockbreak

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/veutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *veutil.Scraper {
	return veutil.New(veutil.SiteConfig{
		ID:             "hugecockbreak",
		Studio:         "HugeCockBreak",
		SiteBase:       "https://hugecockbreak.com",
		MainCategoryID: 1,
		Patterns:       []string{"hugecockbreak.com"},
		MatchRe:        regexp.MustCompile(`^https?://(?:www\.)?hugecockbreak\.com(/|$)`),
	})
}

func init() { scraper.Register(New()) }
