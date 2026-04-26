package boyfriendsharing

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/veutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *veutil.Scraper {
	return veutil.New(veutil.SiteConfig{
		ID:             "boyfriendsharing",
		Studio:         "ShareMyBF",
		SiteBase:       "https://boyfriendsharing.com",
		MainCategoryID: 1,
		Patterns:       []string{"boyfriendsharing.com"},
		MatchRe:        regexp.MustCompile(`^https?://(?:www\.)?boyfriendsharing\.com(/|$)`),
	})
}

func init() { scraper.Register(New()) }
