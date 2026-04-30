package moodyz

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/uptimelyutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *uptimelyutil.Scraper {
	return uptimelyutil.New(uptimelyutil.SiteConfig{
		ID:     "moodyz",
		Studio: "MOODYZ",
		Domain: "moodyz.com",
		Patterns: []string{
			"moodyz.com/works/list/series/{id}",
			"moodyz.com/works/list/release",
			"moodyz.com/works/list/date/{date}",
			"moodyz.com/works/list/genre/{id}",
			"moodyz.com/works/list/label/{id}",
			"moodyz.com/actress/detail/{id}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?moodyz\.com/(?:works/list/|actress/detail/)`),
	})
}

func init() { scraper.Register(New()) }
