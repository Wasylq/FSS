// Package humiliationpov registers the Humiliation POV scraper. The WordPress
// install lives under /blog/, so its REST API is at /blog/wp-json. Scenes are
// `post` objects. See fotoroutil.
package humiliationpov

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/fotoroutil"
	"github.com/Wasylq/FSS/scraper"
)

func init() {
	scraper.Register(fotoroutil.New(fotoroutil.SiteConfig{
		ID:         "humiliationpov",
		Studio:     "Humiliation POV",
		SiteBase:   "https://www.humiliationpov.com/blog",
		TagsAsTags: true,
		Patterns:   []string{"humiliationpov.com", "humiliationpov.com/blog/tag/{slug}"},
		MatchRe:    regexp.MustCompile(`^https?://(?:www\.)?humiliationpov\.com`),
	}))
}
