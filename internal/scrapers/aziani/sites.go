package aziani

import (
	"regexp"

	"github.com/Wasylq/FSS/scraper"
)

// sites is the table of Aziani network sites. cms_area_id UUIDs are pulled
// from each site's `/natscms-app/config.json`. All four sites share a single
// content pool (~3300 scenes) via azianistudios.com.
//
// Dead/redirected sites not covered:
//   - azianixposed.com — 301 → aziani.com
//   - povtrain.com — connection refused
//   - clubheathersummers.com — connection refused
var sites = []SiteConfig{
	{
		ID:        "aziani",
		SiteBase:  "https://www.aziani.com",
		SiteName:  "Aziani",
		CMSAreaID: "3b4c609c-6a0d-4cb9-9cce-0605f32b79ec",
		Patterns:  []string{"aziani.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?aziani\.com\b`),
	},
	{
		ID:        "2poles1hole",
		SiteBase:  "https://2poles1hole.com",
		SiteName:  "2 Poles 1 Hole",
		CMSAreaID: "4f0a134f-8a1b-4bcb-8013-7e02fac4f61d",
		Patterns:  []string{"2poles1hole.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?2poles1hole\.com\b`),
	},
	{
		ID:        "creampiled",
		SiteBase:  "https://creampiled.com",
		SiteName:  "CreamPiled",
		CMSAreaID: "29c61dae-db14-419b-93a4-d016b928dee9",
		Patterns:  []string{"creampiled.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?creampiled\.com\b`),
	},
	{
		ID:        "popuporgies",
		SiteBase:  "https://popuporgies.com",
		SiteName:  "Pop Up Orgies",
		CMSAreaID: "ae0a26fe-7f08-433d-bb04-a9b6f358c48e",
		Patterns:  []string{"popuporgies.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?popuporgies\.com\b`),
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(New(cfg))
	}
}
