package s1no1style

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/uptimelyutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *uptimelyutil.Scraper {
	return uptimelyutil.New(uptimelyutil.SiteConfig{
		ID:     "s1no1style",
		Studio: "S1 NO.1 STYLE",
		Domain: "s1s1s1.com",
		Patterns: []string{
			"s1s1s1.com/works/list/series/{id}",
			"s1s1s1.com/works/list/release",
			"s1s1s1.com/works/list/date/{date}",
			"s1s1s1.com/works/list/genre/{id}",
			"s1s1s1.com/works/list/label/{id}",
			"s1s1s1.com/actress/detail/{id}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?s1s1s1\.com/(?:works/list/|actress/detail/)`),
	})
}

func init() { scraper.Register(New()) }
