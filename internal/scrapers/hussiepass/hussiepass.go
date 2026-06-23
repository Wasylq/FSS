// Package hussiepass registers the Hussie Pass network sites (povporncash NATS
// tour template). hussieauditions.com is omitted: its TLS certificate is
// expired, so it is unreachable. See hussieutil.
package hussiepass

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/hussieutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []hussieutil.SiteConfig{
	{
		ID:       "hussiepass",
		Studio:   "Hussie Pass",
		SiteBase: "https://hussiepass.com",
		Patterns: []string{"hussiepass.com", "hussiepass.com/categories/movies/{N}/latest/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?hussiepass\.com`),
	},
	{
		ID:         "povpornstars",
		Studio:     "POV Pornstars",
		SiteBase:   "https://www.povpornstars.com",
		TourPrefix: "/tour",
		Patterns:   []string{"povpornstars.com", "povpornstars.com/tour/categories/movies/{N}/latest/"},
		MatchRe:    regexp.MustCompile(`^https?://(?:www\.)?povpornstars\.com`),
	},
	{
		ID:       "interracialpovs",
		Studio:   "Interracial POVs",
		SiteBase: "https://interracialpovs.com",
		Patterns: []string{"interracialpovs.com", "interracialpovs.com/categories/movies/{N}/latest/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?interracialpovs\.com`),
	},
	{
		ID:       "hotandtatted",
		Studio:   "Hot and Tatted",
		SiteBase: "https://hotandtatted.com",
		Patterns: []string{"hotandtatted.com", "hotandtatted.com/categories/movies/{N}/latest/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?hotandtatted\.com`),
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(hussieutil.New(cfg))
	}
}
