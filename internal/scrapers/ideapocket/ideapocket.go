package ideapocket

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/uptimelyutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *uptimelyutil.Scraper {
	return uptimelyutil.New(uptimelyutil.SiteConfig{
		ID:     "ideapocket",
		Studio: "Idea Pocket",
		Domain: "ideapocket.com",
		Patterns: []string{
			"ideapocket.com/works/list/series/{id}",
			"ideapocket.com/works/list/release",
			"ideapocket.com/works/list/date/{date}",
			"ideapocket.com/works/list/genre/{id}",
			"ideapocket.com/works/list/label/{id}",
			"ideapocket.com/actress/detail/{id}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?ideapocket\.com/(?:works/list/|actress/detail/)`),
	})
}

func init() { scraper.Register(New()) }
