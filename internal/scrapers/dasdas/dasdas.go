package dasdas

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/uptimelyutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *uptimelyutil.Scraper {
	return uptimelyutil.New(uptimelyutil.SiteConfig{
		ID:     "dasdas",
		Studio: "DAS!",
		Domain: "dasdas.jp",
		Patterns: []string{
			"dasdas.jp/works/list/series/{id}",
			"dasdas.jp/works/list/release",
			"dasdas.jp/works/list/date/{date}",
			"dasdas.jp/works/list/genre/{id}",
			"dasdas.jp/works/list/label/{id}",
			"dasdas.jp/actress/detail/{id}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?dasdas\.jp/(?:works/list/|actress/detail/)`),
	})
}

func init() { scraper.Register(New()) }
