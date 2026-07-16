// Package brutalmaster registers the Brutal Master scraper. The bondage site
// runs WordPress with an open REST API; scenes are `post` objects (titles
// encode date+model+scene). See fotoroutil.
package brutalmaster

import (
	"regexp"
	"time"

	"github.com/Wasylq/FSS/internal/scrapers/fotoroutil"
	"github.com/Wasylq/FSS/scraper"
)

// config is the single source of truth for this site, so the integration test
// exercises the same settings that ship — notably the raised Timeout.
func config() fotoroutil.SiteConfig {
	return fotoroutil.SiteConfig{
		ID:         "brutalmaster",
		Studio:     "Brutal Master",
		SiteBase:   "https://brutalmaster.com",
		TagsAsTags: true,
		// The WP REST listing returns ~4 MB per 100-post page and takes about
		// 40s to respond, so the util's 30s default times out on every run.
		Timeout:  2 * time.Minute,
		Patterns: []string{"brutalmaster.com", "brutalmaster.com/tag/{slug}"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?brutalmaster\.com`),
	}
}

// New constructs the Brutal Master scraper.
func New() *fotoroutil.Scraper { return fotoroutil.New(config()) }

func init() { scraper.Register(New()) }
