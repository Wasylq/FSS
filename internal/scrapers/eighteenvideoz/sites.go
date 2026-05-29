package eighteenvideoz

import (
	"regexp"

	"github.com/Wasylq/FSS/scraper"
)

// sites enumerates every Wow-Tube sister site we can reach with one of
// the four card-format variants that parseListing handles. Out-of-scope
// sites:
//
//   - 18flesh.com, homepornotapes.com, ishootmygirl.com, mydirtygf.com,
//     pornfilms3d.com, nasty-angels.com, girlsnextdoorabused.com —
//     landing/promo pages with no scene cards.
//
// Note these legacy sites are HTTP-only (HTTPS doesn't resolve), so
// SiteBase is `http://…` for the Variant D entries.
var sites = []SiteConfig{
	{
		// Parent portal — aggregates scenes from every sub-site, each
		// card carries its source in `<span class="author">` which the
		// parser lifts onto Scene.Series, overriding the empty SiteName.
		ID:             "18videoz",
		SiteBase:       "https://18videoz.com",
		SiteName:       "", // per-card label takes precedence
		PaginationPath: "/index.php/main/show_sets2",
		Patterns: []string{
			"18videoz.com/",
			"18videoz.com/index.php/main/show_sets2/{N}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?18videoz\.com(?:/|$)`),
	},
	{
		ID:             "casualteensex",
		SiteBase:       "https://casualteensex.com",
		SiteName:       "Casual Teen Sex",
		PaginationPath: "/index.php/main/show_sets2",
		Patterns: []string{
			"casualteensex.com/",
			"casualteensex.com/index.php/main/show_sets2/{N}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?casualteensex\.com(?:/|$)`),
	},
	{
		ID:             "sellyourgf",
		SiteBase:       "https://sellyourgf.com",
		SiteName:       "Sell Your GF",
		PaginationPath: "/index.php/main/show_sets2",
		Patterns: []string{
			"sellyourgf.com/",
			"sellyourgf.com/index.php/main/show_sets2/{N}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?sellyourgf\.com(?:/|$)`),
	},
	{
		ID:             "youngsexparties",
		SiteBase:       "https://youngsexparties.com",
		SiteName:       "Young Sex Parties",
		PaginationPath: "/index.php/main/show_sets2",
		Patterns: []string{
			"youngsexparties.com/",
			"youngsexparties.com/index.php/main/show_sets2/{N}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?youngsexparties\.com(?:/|$)`),
	},
	{
		// Single-page: homepage is the full catalogue (~280 cards).
		ID:             "teensanalyzed",
		SiteBase:       "https://teensanalyzed.com",
		SiteName:       "Teens Analyzed",
		PaginationPath: "",
		Patterns:       []string{"teensanalyzed.com/"},
		MatchRe:        regexp.MustCompile(`^https?://(?:www\.)?teensanalyzed\.com(?:/|$)`),
	},
	{
		// teenylovers uses /show_sets/{N} (no `2`) for pagination and the
		// older `<div id="N" class="th">` + `re_add_click('N', '…')` card
		// variant. Same parser still works thanks to the relaxed regexes.
		ID:             "teenylovers",
		SiteBase:       "https://teenylovers.com",
		SiteName:       "Teeny Lovers",
		PaginationPath: "/index.php/main/show_sets",
		Patterns: []string{
			"teenylovers.com/",
			"teenylovers.com/index.php/main/show_sets/{N}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?teenylovers\.com(?:/|$)`),
	},
	{
		// younglibertines uses Variant C (<div class="thumb"> + show_trailer).
		// Pagination via /show_sets/{N} (no `2`).
		ID:             "younglibertines",
		SiteBase:       "https://younglibertines.com",
		SiteName:       "Young Libertines",
		PaginationPath: "/index.php/main/show_sets",
		Patterns: []string{
			"younglibertines.com/",
			"younglibertines.com/index.php/main/show_sets/{N}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?younglibertines\.com(?:/|$)`),
	},

	// --- Variant D: legacy static-HTML table layout ---
	// HTTP-only (no HTTPS support on origin); PaginationPath uses "{N}"
	// because page-N is a separate file name `indexN.htm` rather than a
	// path suffix.
	{
		ID:             "bangmyteenass",
		SiteBase:       "http://bangmyteenass.com",
		SiteName:       "Bang My Teen Ass",
		PaginationPath: "/index{N}.htm",
		Patterns: []string{
			"bangmyteenass.com/",
			"bangmyteenass.com/index{N}.htm",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?bangmyteenass\.com(?:/|$)`),
	},
	{
		ID:             "firstanaldate",
		SiteBase:       "http://firstanaldate.com",
		SiteName:       "First Anal Date",
		PaginationPath: "/index{N}.htm",
		Patterns: []string{
			"firstanaldate.com/",
			"firstanaldate.com/index{N}.htm",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?firstanaldate\.com(?:/|$)`),
	},
	{
		ID:             "olddicksyoungchix",
		SiteBase:       "http://olddicksyoungchix.com",
		SiteName:       "Old Dicks Young Chix",
		PaginationPath: "/index{N}.htm",
		Patterns: []string{
			"olddicksyoungchix.com/",
			"olddicksyoungchix.com/index{N}.htm",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?olddicksyoungchix\.com(?:/|$)`),
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(New(cfg))
	}
}
