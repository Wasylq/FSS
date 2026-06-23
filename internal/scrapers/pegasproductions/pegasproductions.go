// Package pegasproductions registers the Pegas Productions scraper. The Quebec
// studio runs vanilla WordPress with an open REST API; scenes are `post`
// objects. See fotoroutil.
package pegasproductions

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/fotoroutil"
	"github.com/Wasylq/FSS/scraper"
)

func init() {
	scraper.Register(fotoroutil.New(fotoroutil.SiteConfig{
		ID:         "pegasproductions",
		Studio:     "Pegas Productions",
		SiteBase:   "https://www.pegasproductions.com",
		TagsAsTags: true,
		Patterns:   []string{"pegasproductions.com", "pegasproductions.com/tag/{slug}"},
		MatchRe:    regexp.MustCompile(`^https?://(?:www\.)?pegasproductions\.com`),
	}))
}
