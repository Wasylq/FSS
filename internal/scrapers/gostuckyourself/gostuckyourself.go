package gostuckyourself

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/veutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *veutil.Scraper {
	return veutil.New(veutil.SiteConfig{
		ID:             "gostuckyourself",
		Studio:         "GoStuckYourself",
		SiteBase:       "https://gostuckyourself.net",
		MainCategoryID: 1,
		Patterns:       []string{"gostuckyourself.net"},
		MatchRe:        regexp.MustCompile(`^https?://(?:www\.)?gostuckyourself\.net(/|$)`),
	})
}

func init() { scraper.Register(New()) }
