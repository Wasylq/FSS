// Package maycontaingirl registers the May Contain Girl scraper. The art-nude
// site runs WordPress with an open REST API; scenes/issues are `post` objects.
// See fotoroutil.
package maycontaingirl

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/fotoroutil"
	"github.com/Wasylq/FSS/scraper"
)

func init() {
	scraper.Register(fotoroutil.New(fotoroutil.SiteConfig{
		ID:         "maycontaingirl",
		Studio:     "May Contain Girl",
		SiteBase:   "https://maycontaingirl.com",
		TagsAsTags: true,
		Patterns:   []string{"maycontaingirl.com", "maycontaingirl.com/tag/{slug}"},
		MatchRe:    regexp.MustCompile(`^https?://(?:www\.)?maycontaingirl\.com`),
	}))
}
