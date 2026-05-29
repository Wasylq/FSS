package wtfpass

import (
	"regexp"

	"github.com/Wasylq/FSS/scraper"
)

// sites enumerates every WTFPass network site. The parent wtfpass.com
// aggregates the entire 6,000-scene catalogue (85 pages × 72 cards), so
// scraping the parent ID alone covers the network — each card's
// `<span class="site">` overrides the empty `SiteName` and labels the
// scene with its true brand. Sister domains scrape only that brand's
// content (the parent's `/sites/{slug}/` filter gives the same view).
var sites = []SiteConfig{
	{
		// Parent — empty SiteName because per-card site labels carry the
		// authoritative brand name for every scene.
		ID:       "wtfpass",
		SiteBase: "https://wtfpass.com",
		SiteName: "",
		Patterns: []string{"wtfpass.com/", "wtfpass.com/videos/{N}/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?wtfpass\.com(?:/|$)`),
	},
	{
		ID:       "cashforsextape",
		SiteBase: "https://cashforsextape.com",
		SiteName: "Cash for Sextape",
		Patterns: []string{"cashforsextape.com/", "cashforsextape.com/videos/{N}/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?cashforsextape\.com(?:/|$)`),
	},
	{
		// College Fuck Parties has two stashdb URLs (collegefuckparties.com
		// and studentsexparties.com) — we register the canonical, the
		// other is registered separately as "Student Sex Parties" below.
		ID:       "collegefuckparties",
		SiteBase: "https://collegefuckparties.com",
		SiteName: "College Fuck Parties",
		Patterns: []string{"collegefuckparties.com/", "collegefuckparties.com/videos/{N}/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?collegefuckparties\.com(?:/|$)`),
	},
	{
		ID:       "dollsporn",
		SiteBase: "https://dollsporn.com",
		SiteName: "Dolls Porn",
		Patterns: []string{"dollsporn.com/", "dollsporn.com/videos/{N}/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?dollsporn\.com(?:/|$)`),
	},
	{
		ID:       "hdmassageporn",
		SiteBase: "https://hdmassageporn.com",
		SiteName: "HD Massage Porn",
		Patterns: []string{"hdmassageporn.com/", "hdmassageporn.com/videos/{N}/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?hdmassageporn\.com(?:/|$)`),
	},
	{
		ID:       "hdsex18",
		SiteBase: "https://hdsex18.com",
		SiteName: "HD Sex 18",
		Patterns: []string{"hdsex18.com/", "hdsex18.com/videos/{N}/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?hdsex18\.com(?:/|$)`),
	},
	{
		ID:       "hardfuckgirls",
		SiteBase: "https://hardfuckgirls.com",
		SiteName: "Hard Fuck Girls",
		Patterns: []string{"hardfuckgirls.com/", "hardfuckgirls.com/videos/{N}/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?hardfuckgirls\.com(?:/|$)`),
	},
	{
		// Hard Fuck Girls has an alternate domain — hardfucktales.com.
		ID:       "hardfucktales",
		SiteBase: "https://hardfucktales.com",
		SiteName: "Hard Fuck Tales",
		Patterns: []string{"hardfucktales.com/", "hardfucktales.com/videos/{N}/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?hardfucktales\.com(?:/|$)`),
	},
	{
		ID:       "hersexdebut",
		SiteBase: "https://hersexdebut.com",
		SiteName: "Her Sex Debut",
		Patterns: []string{"hersexdebut.com/", "hersexdebut.com/videos/{N}/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?hersexdebut\.com(?:/|$)`),
	},
	{
		ID:       "meetsuckandfuck",
		SiteBase: "https://meetsuckandfuck.com",
		SiteName: "Meet Suck and Fuck",
		Patterns: []string{"meetsuckandfuck.com/", "meetsuckandfuck.com/videos/{N}/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?meetsuckandfuck\.com(?:/|$)`),
	},
	{
		ID:       "mypickupgirls",
		SiteBase: "https://mypickupgirls.com",
		SiteName: "My Pickup Girls",
		Patterns: []string{"mypickupgirls.com/", "mypickupgirls.com/videos/{N}/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?mypickupgirls\.com(?:/|$)`),
	},
	{
		ID:       "pandafuck",
		SiteBase: "https://pandafuck.com",
		SiteName: "Panda Fuck",
		Patterns: []string{"pandafuck.com/", "pandafuck.com/videos/{N}/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?pandafuck\.com(?:/|$)`),
	},
	{
		ID:       "pickupfuck",
		SiteBase: "https://pickupfuck.com",
		SiteName: "Pickup Fuck",
		Patterns: []string{"pickupfuck.com/", "pickupfuck.com/videos/{N}/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?pickupfuck\.com(?:/|$)`),
	},
	{
		ID:       "privatesextapes",
		SiteBase: "https://privatesextapes.com",
		SiteName: "Private Sex Tapes",
		Patterns: []string{"privatesextapes.com/", "privatesextapes.com/videos/{N}/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?privatesextapes\.com(?:/|$)`),
	},
	{
		ID:       "publicsexadventures",
		SiteBase: "https://publicsexadventures.com",
		SiteName: "Public Sex Adventures",
		Patterns: []string{"publicsexadventures.com/", "publicsexadventures.com/videos/{N}/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?publicsexadventures\.com(?:/|$)`),
	},
	{
		ID:       "studentsexparties",
		SiteBase: "https://studentsexparties.com",
		SiteName: "Student Sex Parties",
		Patterns: []string{"studentsexparties.com/", "studentsexparties.com/videos/{N}/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?studentsexparties\.com(?:/|$)`),
	},
	{
		ID:       "theartporn",
		SiteBase: "https://theartporn.com",
		SiteName: "The Art Porn",
		Patterns: []string{"theartporn.com/", "theartporn.com/videos/{N}/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?theartporn\.com(?:/|$)`),
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(New(cfg))
	}
}
