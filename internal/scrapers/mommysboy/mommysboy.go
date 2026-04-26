package mommysboy

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/veutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *veutil.Scraper {
	return veutil.New(veutil.SiteConfig{
		ID:             "mommysboy",
		Studio:         "MommysBoy",
		SiteBase:       "https://mommysboy.net",
		MainCategoryID: 1,
		Patterns:       []string{"mommysboy.net"},
		MatchRe:        regexp.MustCompile(`^https?://(?:www\.)?mommysboy\.net(/|$)`),
	})
}

func init() { scraper.Register(New()) }
