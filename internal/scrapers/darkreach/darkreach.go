// Package darkreach registers scrapers for Darkreach Communications sister
// sites. The network's children use a wide variety of NATS templates, so
// this single config package routes each site to the right parser util.
//
// Coverage map:
//
//   - darkreachmodernutil      → mybestsexlife, givingahandjob, erotiquetvlive
//   - darkreachupdateitemutil  → spartavideo, watchyoujerk, angelasommers
//   - adultdoorwayclassicutil  → babearchives (now with TourPrefix="" support)
//
// Sites with their own standalone scrapers (registered elsewhere):
//
//   - collegeuniform (data-setid update_details template)
package darkreach

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/adultdoorwayclassicutil"
	"github.com/Wasylq/FSS/internal/scrapers/darkreachmodernutil"
	"github.com/Wasylq/FSS/internal/scrapers/darkreachupdateitemutil"
	"github.com/Wasylq/FSS/internal/scrapers/darkreachupdatesutil"
	"github.com/Wasylq/FSS/scraper"
)

// Modern Bootstrap-grid template (item-update item-video).
var modernSites = []darkreachmodernutil.SiteConfig{
	{
		ID:       "mybestsexlife",
		SiteBase: "https://www.mybestsexlife.com",
		Studio:   "My Best Sex Life",
		Patterns: []string{"mybestsexlife.com", "mybestsexlife.com/categories/movies_{N}_d.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?mybestsexlife\.com`),
	},
	{
		ID:       "givingahandjob",
		SiteBase: "https://givingahandjob.com",
		Studio:   "Giving A Handjob",
		Patterns: []string{"givingahandjob.com", "givingahandjob.com/categories/movies_{N}_d.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?givingahandjob\.com`),
	},
	{
		ID:         "erotiquetvlive",
		SiteBase:   "https://erotiquetvlive.com",
		Studio:     "ErotiqueTVLive",
		TourPrefix: "/tour",
		Patterns:   []string{"erotiquetvlive.com", "erotiquetvlive.com/tour/categories/movies_{N}_d.html"},
		MatchRe:    regexp.MustCompile(`^https?://(?:www\.)?erotiquetvlive\.com`),
	},
	{
		// GirlsKissXXX is the rebranded iKissGirls — the latter's homepage now
		// redirects users here. Both URL patterns route to this scraper.
		ID:       "girlskissxxx",
		SiteBase: "https://girlskissxxx.com",
		Studio:   "iKissGirls",
		Patterns: []string{"girlskissxxx.com", "ikissgirls.com", "girlskissxxx.com/categories/movies_{N}_d.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?(?:girlskissxxx|ikissgirls)\.com`),
	},
}

// updateItem template (h4/h5 + opt. duration/date/performers).
var updateItemSites = []darkreachupdateitemutil.SiteConfig{
	{
		ID:       "spartavideo",
		SiteBase: "https://spartavideo.com",
		Studio:   "Sparta Video",
		Patterns: []string{"spartavideo.com", "spartavideo.com/categories/updates_{N}_d.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?spartavideo\.com`),
	},
	{
		// www.hammerboys.tv returns an 8-byte stub — use the bare domain.
		ID:       "hammerboys",
		SiteBase: "https://hammerboys.tv",
		Studio:   "HammerBoys.tv",
		Patterns: []string{"hammerboys.tv", "hammerboys.tv/categories/updates_{N}_d.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?hammerboys\.tv`),
	},
	{
		ID:       "watchyoujerk",
		SiteBase: "http://watchyoujerk.com",
		Studio:   "Watch You Jerk",
		Patterns: []string{"watchyoujerk.com", "watchyoujerk.com/categories/updates_{N}_d.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?watchyoujerk\.com`),
	},
	{
		ID:       "angelasommers",
		SiteBase: "https://www.angelasommers.com",
		Studio:   "Angela Sommers",
		Patterns: []string{"angelasommers.com", "angelasommers.com/categories/updates_{N}_d.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?angelasommers\.com`),
	},
	{
		ID:         "hungarianhoneys",
		SiteBase:   "http://www.hungarianhoneys.com",
		Studio:     "Hungarian Honeys",
		TourPrefix: "/tour",
		Patterns:   []string{"hungarianhoneys.com", "hungarianhoneys.com/tour/categories/updates_{N}_d.html"},
		MatchRe:    regexp.MustCompile(`^https?://(?:www\.)?hungarianhoneys\.com`),
	},
	{
		ID:                "terrorxxx",
		SiteBase:          "https://terrorxxx.com",
		Studio:            "TerrorXXX",
		DetailPathSegment: "/trailers",
		ListingBase:       "/categories/Movies",
		Patterns:          []string{"terrorxxx.com", "terrorxxx.com/categories/Movies_{N}_d.html"},
		MatchRe:           regexp.MustCompile(`^https?://(?:www\.)?terrorxxx\.com`),
	},
}

// Affiliate marketing template — "updates clear" wrapper with h3 title and
// description, scene anchors point at /signup.php?nats=…, no detail pages.
var updatesMarketingSites = []darkreachupdatesutil.SiteConfig{
	{
		ID:       "clubkayden",
		SiteBase: "https://clubkayden.com",
		Studio:   "Club Kayden",
		Patterns: []string{"clubkayden.com", "clubkayden.com/updates/page_{N}.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?clubkayden\.com`),
	},
	{
		ID:       "thelisaann",
		SiteBase: "https://www.thelisaann.com",
		Studio:   "Lisa Ann",
		Patterns: []string{"thelisaann.com", "thelisaann.com/updates/page_{N}.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?thelisaann\.com`),
	},
	{
		ID:       "theavaaddams",
		SiteBase: "https://www.theavaaddams.com",
		Studio:   "Ava Addams",
		Patterns: []string{"theavaaddams.com", "theavaaddams.com/updates/page_{N}.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?theavaaddams\.com`),
	},
	{
		ID:       "kortneykane",
		SiteBase: "http://kortneykane.com",
		Studio:   "Kortney Kane",
		Patterns: []string{"kortneykane.com", "kortneykane.com/updates/page_{N}.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?kortneykane\.com`),
	},
}

// Adult Doorway Classic (item-thumb cards, /categories/movies/{N}/latest/).
var classicSites = []adultdoorwayclassicutil.SiteConfig{
	{
		ID:       "babearchives",
		SiteBase: "https://www.babearchives.com",
		Studio:   "Babe Archives",
		// TourPrefix="" — babearchives serves listings at the bare path,
		// unlike blackpayback which uses /tour/.
		Patterns: []string{"babearchives.com", "babearchives.com/categories/movies/{N}/latest/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?babearchives\.com`),
	},
}

func init() {
	for _, cfg := range modernSites {
		scraper.Register(darkreachmodernutil.New(cfg))
	}
	for _, cfg := range updateItemSites {
		scraper.Register(darkreachupdateitemutil.New(cfg))
	}
	for _, cfg := range updatesMarketingSites {
		scraper.Register(darkreachupdatesutil.New(cfg))
	}
	for _, cfg := range classicSites {
		scraper.Register(adultdoorwayclassicutil.New(cfg))
	}
}
