package feetondemand

import (
	"regexp"

	"github.com/Wasylq/FSS/scraper"
)

// sites lists the 5 Feet on Demand sister tours that expose a
// scrapeable Videos catalogue via the AJAX template. Sites not covered:
//
//   - feetpov.com, footfetishpetite.com, goddessfootworship.com,
//     goddessfootjobs.com — pure marketing splashes whose nav links
//     all redirect to the join page; no scenes accessible.
//   - fetishcustoms.com — different custom-clip template with only
//     ~10 scenes featured on the home page; not worth a separate parser.
//   - footslaveauditions.com — 404 dead.
//   - feetondemand.com (parent) — redirects to an AI-generated
//     manus.space page, not a real catalogue.
var sites = []SiteConfig{
	{
		ID:       "goddessfootdomination",
		BaseURL:  "https://www.goddessfootdomination.com",
		SiteName: "Goddess Foot Domination",
		Patterns: []string{"https://www.goddessfootdomination.com/"},
		MatchRe:  regexp.MustCompile(`(?i)^https?://(?:www\.)?goddessfootdomination\.com\b`),
	},
	{
		ID:       "jerktomyfeet",
		BaseURL:  "https://www.jerktomyfeet.com",
		SiteName: "Jerk to My Feet",
		Patterns: []string{"https://www.jerktomyfeet.com/"},
		MatchRe:  regexp.MustCompile(`(?i)^https?://(?:www\.)?jerktomyfeet\.com\b`),
	},
	{
		ID:       "footfetishcardates",
		BaseURL:  "https://www.footfetishcardates.com",
		SiteName: "Foot Fetish Car Dates",
		Patterns: []string{"https://www.footfetishcardates.com/"},
		MatchRe:  regexp.MustCompile(`(?i)^https?://(?:www\.)?footfetishcardates\.com\b`),
	},
	{
		ID:       "footfetishaffiliates",
		BaseURL:  "https://www.footfetishaffiliates.com",
		SiteName: "Foot Fetish Affiliates",
		Patterns: []string{"https://www.footfetishaffiliates.com/", "https://footfetishaffiliates.com/"},
		MatchRe:  regexp.MustCompile(`(?i)^https?://(?:www\.)?footfetishaffiliates\.com\b`),
	},
	{
		ID:       "goddessbrianna",
		BaseURL:  "https://www.goddessbrianna.net",
		SiteName: "Goddess Brianna",
		Patterns: []string{"https://www.goddessbrianna.net/"},
		MatchRe:  regexp.MustCompile(`(?i)^https?://(?:www\.)?goddessbrianna\.net\b`),
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(New(cfg))
	}
}
