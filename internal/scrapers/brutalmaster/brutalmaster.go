// Package brutalmaster registers the Brutal Master scraper. The bondage site
// runs WordPress with an open REST API; scenes are `post` objects (titles
// encode date+model+scene). See fotoroutil.
package brutalmaster

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/fotoroutil"
	"github.com/Wasylq/FSS/scraper"
)

func init() {
	scraper.Register(fotoroutil.New(fotoroutil.SiteConfig{
		ID:         "brutalmaster",
		Studio:     "Brutal Master",
		SiteBase:   "https://brutalmaster.com",
		TagsAsTags: true,
		Patterns:   []string{"brutalmaster.com", "brutalmaster.com/tag/{slug}"},
		MatchRe:    regexp.MustCompile(`^https?://(?:www\.)?brutalmaster\.com`),
	}))
}
