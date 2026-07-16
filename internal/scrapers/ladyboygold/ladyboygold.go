// Package ladyboygold registers the ladyboygold.com scraper. The site runs the
// same TooMuchMedia NATS tour CMS as Cosplayground and the Mars Media network
// (Angular SPA at `/natscms-app/`, `tour_api.php` JSON API), on the shared
// Island Dollars backend. All discovery and parsing is the natscmsutil core;
// this package only supplies the site's SiteConfig.
//
// cms_area_id and natsUrl come from
// https://www.ladyboygold.com/natscms-app/config.json.
//
// One site-specific wrinkle: the set_list block mixes photo sets into the same
// response as videos — about 1800 of ~5000 entries — with no type field to tell
// them apart. The content `path` is the only signal, so SkipPathRe drops any
// set filed under a photo directory (`photos`, `4kphotos`, `photos4k`,
// `candid_photos`).
package ladyboygold

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/natscmsutil"
	"github.com/Wasylq/FSS/scraper"
)

// skipPathRe matches the photo-set directories in the content path, e.g.
// "lbg/00lfb/4kphotos/some_set/". Video directories (videos, 4kvideos,
// videos4k, remastered4k, ladyboys, …) are kept.
var skipPathRe = regexp.MustCompile(`/[0-9a-z_]*photos[0-9a-z_]*/`)

func New() *natscmsutil.Scraper {
	return natscmsutil.New(natscmsutil.SiteConfig{
		ID:          "ladyboygold",
		SiteBase:    "https://ladyboygold.com",
		SiteName:    "Ladyboy Gold",
		StudioName:  "Ladyboy Gold",
		NatsAPIBase: "https://nats.islanddollars.com/tour_api.php",
		CMSAreaID:   "cd9a5600-5cda-4ed0-b356-f62af1887d96",
		SkipPathRe:  skipPathRe,
		Patterns:    []string{"ladyboygold.com"},
		MatchRe:     regexp.MustCompile(`^https?://(?:www\.)?ladyboygold\.com\b`),
	})
}

func init() { scraper.Register(New()) }
