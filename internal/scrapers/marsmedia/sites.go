// Package marsmedia registers the Mars Media gay-network sister sites that
// run on the My Gay Cash NATS CMS (nats.mygaycash.com). It is a thin site
// table over the shared natscmsutil core; 12 of the 14 stashdb children
// share this platform, while the remaining two (tgirlplaytime.com,
// twotgirls.com) use Nebula CMS and are not yet covered.
package marsmedia

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/natscmsutil"
	"github.com/Wasylq/FSS/scraper"
)

const (
	// mygaycashAPIBase is the shared NATS backend every Mars Media sister
	// site talks to (per-site scoping comes from the CMSAreaID header).
	mygaycashAPIBase = "https://nats.mygaycash.com/tour_api.php"
	marsMediaStudio  = "Mars Media"
)

// sites is the table of Mars Media sister sites covered by this package.
// cms_area_id UUIDs are pulled from each site's `/natscms-app/config.json`
// (the per-site SPA bootstrap config). The 14-child stashdb tree includes
// two extra domains — `tgirlplaytime.com` and `twotgirls.com` — that run a
// different (Nebula) CMS and are not covered here.
var sites = []natscmsutil.SiteConfig{
	{
		ID:        "bearfilms",
		SiteBase:  "https://www.bearfilms.com",
		SiteName:  "Bear Films",
		CMSAreaID: "30e1a84e-91f7-436c-bb1b-d51a8bb521c6",
		Patterns:  []string{"https://www.bearfilms.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.|tour\.)?bearfilms\.com\b`),
	},
	{
		ID:        "barebackcumpigs",
		SiteBase:  "https://www.barebackcumpigs.com",
		SiteName:  "Bareback Cum Pigs",
		CMSAreaID: "1fa10fa0-ddb8-418a-8ed1-f621f19f621c",
		Patterns:  []string{"https://www.barebackcumpigs.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.|tour\.)?barebackcumpigs\.com\b`),
	},
	{
		ID:        "barebackthathole",
		SiteBase:  "https://www.barebackthathole.com",
		SiteName:  "Bareback That Hole",
		CMSAreaID: "b1f1d230-6030-4efd-bcfb-f30650c7675c",
		Patterns:  []string{"https://www.barebackthathole.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.|tour\.)?barebackthathole\.com\b`),
	},
	{
		ID:        "breedmeraw",
		SiteBase:  "https://www.breedmeraw.com",
		SiteName:  "Breed Me Raw",
		CMSAreaID: "e87b32a4-3edf-4c4b-97ee-4fefc2a88a34",
		Patterns:  []string{"https://www.breedmeraw.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.|tour\.)?breedmeraw\.com\b`),
	},
	{
		ID:        "bringmeaboy",
		SiteBase:  "https://www.bringmeaboy.com",
		SiteName:  "Bring Me A Boy",
		CMSAreaID: "6b3ea9d2-0364-4d1b-99f4-9949cca65a59",
		Patterns:  []string{"https://www.bringmeaboy.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.|tour\.)?bringmeaboy\.com\b`),
	},
	{
		ID:        "bulldogpit",
		SiteBase:  "https://www.bulldogpit.com",
		SiteName:  "Bulldog Pit",
		CMSAreaID: "b3981bf3-f23d-44ee-9b9e-5552618d460e",
		Patterns:  []string{"https://www.bulldogpit.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.|tour\.)?bulldogpit\.com\b`),
	},
	{
		ID:        "daddyontwink",
		SiteBase:  "https://www.daddyontwink.com",
		SiteName:  "Daddy On Twink",
		CMSAreaID: "0e703ea1-af51-4d1c-8d74-36b3f051288b",
		Patterns:  []string{"https://www.daddyontwink.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.|tour\.)?daddyontwink\.com\b`),
	},
	{
		ID:        "hairyandraw",
		SiteBase:  "https://www.hairyandraw.com",
		SiteName:  "Hairy And Raw",
		CMSAreaID: "34e58e83-da78-43d7-9606-7fbf34ad08fa",
		Patterns:  []string{"https://www.hairyandraw.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.|tour\.)?hairyandraw\.com\b`),
	},
	{
		ID:        "hardbritlads",
		SiteBase:  "https://www.hardbritlads.com",
		SiteName:  "Hard Brit Lads",
		CMSAreaID: "9baa305b-f3f6-4854-b7e6-b9e0cbabaaad",
		Patterns:  []string{"https://www.hardbritlads.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.|tour\.)?hardbritlads\.com\b`),
	},
	{
		ID:        "southernstrokes",
		SiteBase:  "https://www.southernstrokes.com",
		SiteName:  "Southern Strokes",
		CMSAreaID: "c78e6e09-7276-4c13-9c2e-8f5cf51e8bed",
		Patterns:  []string{"https://www.southernstrokes.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.|tour\.)?southernstrokes\.com\b`),
	},
	{
		ID:        "touchthatboy",
		SiteBase:  "https://www.touchthatboy.com",
		SiteName:  "Touch That Boy",
		CMSAreaID: "e6132199-686e-44b0-a624-a6724c817a35",
		Patterns:  []string{"https://www.touchthatboy.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.|tour\.)?touchthatboy\.com\b`),
	},
	{
		ID:        "twinksinshorts",
		SiteBase:  "https://www.twinksinshorts.com",
		SiteName:  "Twinks In Shorts",
		CMSAreaID: "e78aca5e-111c-44d8-aba5-64994846f223",
		Patterns:  []string{"https://www.twinksinshorts.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.|tour\.)?twinksinshorts\.com\b`),
	},
}

// withDefaults fills the shared Mars Media backend + studio name onto a
// table row (the per-row entries only carry site-specific fields).
func withDefaults(cfg natscmsutil.SiteConfig) natscmsutil.SiteConfig {
	cfg.NatsAPIBase = mygaycashAPIBase
	cfg.StudioName = marsMediaStudio
	return cfg
}

func init() {
	for _, cfg := range sites {
		scraper.Register(natscmsutil.New(withDefaults(cfg)))
	}
}
