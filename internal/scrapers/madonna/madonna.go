package madonna

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/uptimelyutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *uptimelyutil.Scraper {
	return uptimelyutil.New(uptimelyutil.SiteConfig{
		ID:     "madonna",
		Studio: "Madonna",
		Domain: "madonna-av.com",
		Patterns: []string{
			"madonna-av.com/works/list/series/{id}",
			"madonna-av.com/works/list/release",
			"madonna-av.com/works/list/date/{date}",
			"madonna-av.com/works/list/genre/{id}",
			"madonna-av.com/works/list/label/{id}",
			"madonna-av.com/actress/detail/{id}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?madonna-av\.com/(?:works/list/|actress/detail/)`),
	})
}

func init() { scraper.Register(New()) }
