package momxxx

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/veutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *veutil.Scraper {
	return veutil.New(veutil.SiteConfig{
		ID:             "momxxx",
		Studio:         "MomXXX",
		SiteBase:       "https://momxxx.org",
		MainCategoryID: 10,
		Patterns:       []string{"momxxx.org"},
		MatchRe:        regexp.MustCompile(`^https?://(?:www\.)?momxxx\.org(/|$)`),
	})
}

func init() { scraper.Register(New()) }
