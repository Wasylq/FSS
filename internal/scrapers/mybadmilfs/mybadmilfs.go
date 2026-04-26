package mybadmilfs

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/veutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *veutil.Scraper {
	return veutil.New(veutil.SiteConfig{
		ID:             "mybadmilfs",
		Studio:         "MyBadMILFs",
		SiteBase:       "https://mybadmilfs.com",
		MainCategoryID: 1,
		Patterns:       []string{"mybadmilfs.com"},
		MatchRe:        regexp.MustCompile(`^https?://(?:www\.)?mybadmilfs\.com(/|$)`),
	})
}

func init() { scraper.Register(New()) }
