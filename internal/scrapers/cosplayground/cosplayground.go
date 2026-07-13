// Package cosplayground registers the cosplayground.com scraper. The site
// runs the same TooMuchMedia / My Gay Cash NATS tour CMS as the Mars Media
// network (Angular SPA at `/natscms-app/`, `tour_api.php` JSON API), but on
// a self-hosted backend (`api.cosplayground.com`) under a different
// operator (skinfluentialmedia.com). All discovery/parsing is the shared
// natscmsutil core; this package only supplies the site's SiteConfig.
//
// cms_area_id and natsUrl come from https://cosplayground.com/natscms-app/config.json.
package cosplayground

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/natscmsutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *natscmsutil.Scraper {
	return natscmsutil.New(natscmsutil.SiteConfig{
		ID:          "cosplayground",
		SiteBase:    "https://cosplayground.com",
		SiteName:    "Cosplayground",
		StudioName:  "Cosplayground",
		NatsAPIBase: "https://api.cosplayground.com/tour_api.php",
		CMSAreaID:   "2bc2444e-a053-40a2-bc4b-ca3618521563",
		Patterns:    []string{"https://cosplayground.com/"},
		MatchRe:     regexp.MustCompile(`^https?://(?:www\.)?cosplayground\.com\b`),
	})
}

func init() { scraper.Register(New()) }
