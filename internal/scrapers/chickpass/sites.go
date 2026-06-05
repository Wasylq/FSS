package chickpass

import (
	"regexp"

	"github.com/Wasylq/FSS/scraper"
)

// sites is the table of ChickPass Network sites. cms_area_id UUIDs are pulled
// from each site's `/natscms-app/config.json`. All sites share the
// chickpasscash.com NATS API but each has a filtered content pool.
//
// Dead/redirected sites not covered:
//   - tandaasians.com — 302 → chickpass.com
//   - tandabrunettes.com — 301 → chickpass.com
//   - tandalesbians.com — 302 → chickpass.com
//   - tandastudios.com — static personal bio page, not a content site
var sites = []SiteConfig{
	{
		ID:        "chickpass",
		SiteBase:  "https://www.chickpass.com",
		SiteName:  "ChickPass",
		CMSAreaID: "2fac6e56-1fe9-4486-8519-c97affafaea7",
		Patterns:  []string{"chickpass.com/", "chickpassnetwork.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?(?:chickpass|chickpassnetwork)\.com\b`),
	},
	{
		ID:        "bouncychicks",
		SiteBase:  "https://www.bouncychicks.com",
		SiteName:  "Bouncy Chicks",
		CMSAreaID: "6a364df3-82f4-4fa2-be66-88d0d37d3748",
		Patterns:  []string{"bouncychicks.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?bouncychicks\.com\b`),
	},
	{
		ID:        "fuckthegeek",
		SiteBase:  "https://www.fuckthegeek.com",
		SiteName:  "Fuck The Geek",
		CMSAreaID: "d9533f7e-0a4c-43e7-8f24-892ab52cbb8a",
		Patterns:  []string{"fuckthegeek.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?fuckthegeek\.com\b`),
	},
	{
		ID:        "minimuff",
		SiteBase:  "https://www.minimuff.com",
		SiteName:  "Minimuffs",
		CMSAreaID: "99dc9646-54ed-47ea-8a0c-21faced5e12f",
		Patterns:  []string{"minimuff.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?minimuff\.com\b`),
	},
	{
		ID:        "chickpassteens",
		SiteBase:  "https://www.chickpassteens.com",
		SiteName:  "ChickPass Teens",
		CMSAreaID: "9c2cab9b-ef15-4d8d-ae0f-dfc814a27625",
		Patterns:  []string{"chickpassteens.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?chickpassteens\.com\b`),
	},
	{
		ID:        "chickpasslesbians",
		SiteBase:  "https://www.chickpasslesbians.com",
		SiteName:  "ChickPass Lesbians",
		CMSAreaID: "6320bff5-cb14-473b-8a3a-fa774bc33d46",
		Patterns:  []string{"chickpasslesbians.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?chickpasslesbians\.com\b`),
	},
	{
		ID:        "chickpassmilfs",
		SiteBase:  "https://www.chickpassmilfs.com",
		SiteName:  "ChickPass MILFs",
		CMSAreaID: "09fdf9e8-5934-465d-86ca-763ac6d2fb3d",
		Patterns:  []string{"chickpassmilfs.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?chickpassmilfs\.com\b`),
	},
	{
		ID:        "xxxnj",
		SiteBase:  "https://www.xxxnj.com",
		SiteName:  "XXXNJ",
		CMSAreaID: "807b26ac-0d47-4be8-bbea-f7e6efab14a0",
		Patterns:  []string{"xxxnj.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?xxxnj\.com\b`),
	},
	{
		ID:        "fuckingparties",
		SiteBase:  "https://www.fuckingparties.com",
		SiteName:  "Fucking Parties",
		CMSAreaID: "df90e5c7-1cf2-435a-9200-0ac5d85c6456",
		Patterns:  []string{"fuckingparties.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?fuckingparties\.com\b`),
	},
	{
		ID:        "stuffintwats",
		SiteBase:  "https://www.stuffintwats.com",
		SiteName:  "Stuffin' Twats",
		CMSAreaID: "43a1dbd2-b2ec-466a-8ff7-a97dc974fee8",
		Patterns:  []string{"stuffintwats.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?stuffintwats\.com\b`),
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(New(cfg))
	}
}
