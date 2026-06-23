// Package latinboyz registers the LatinBoyz scraper. The site runs vanilla
// WordPress with an open REST API; scenes are `post` objects (the feed also
// includes occasional text "stories"). See fotoroutil.
package latinboyz

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/fotoroutil"
	"github.com/Wasylq/FSS/scraper"
)

func init() {
	scraper.Register(fotoroutil.New(fotoroutil.SiteConfig{
		ID:         "latinboyz",
		Studio:     "LatinBoyz",
		SiteBase:   "https://latinboyz.com",
		TagsAsTags: true,
		Patterns:   []string{"latinboyz.com", "latinboyz.com/tag/{slug}"},
		MatchRe:    regexp.MustCompile(`^https?://(?:www\.)?latinboyz\.com`),
	}))
}
