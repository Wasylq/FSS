package dirtyflix

import (
	"regexp"

	"github.com/Wasylq/FSS/scraper"
)

// sites enumerates every Dirty Flix network site covered by one of the
// three current parser variants. 11 of 16 sister brands still need
// custom parsers (see partially-covered tracking + the package docstring
// for the layout each remaining site uses).
var sites = []SiteConfig{
	{
		// Parent portal — single-page catalogue, per-card sub-brand label.
		ID:        "dirtyflix",
		SiteBase:  "https://dirtyflix.com",
		SiteName:  "Dirty Flix",
		Variant:   VariantThumbsItem,
		Paginated: false,
		Patterns:  []string{"dirtyflix.com/"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?dirtyflix\.com(?:/|$)`),
	},
	{
		// VariantBrutalX — `<div id="N" class="th">` + click_me cards.
		ID:        "brutalx",
		SiteBase:  "https://brutalx.com",
		SiteName:  "Brutal X",
		Variant:   VariantBrutalX,
		Paginated: true,
		Patterns:  []string{"brutalx.com/", "brutalx.com/index.php/main/show_sets2/{N}"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?brutalx\.com(?:/|$)`),
	},
	{
		// VariantThumbWrap — `<a class="thumb_wrap">` + re_add_click cards,
		// caption-text title flavour.
		ID:        "kinkyfamily",
		SiteBase:  "https://kinkyfamily.com",
		SiteName:  "Kinky Family",
		Variant:   VariantThumbWrap,
		Paginated: true,
		Patterns:  []string{"kinkyfamily.com/", "kinkyfamily.com/index.php/main/show_sets2/{N}"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?kinkyfamily\.com(?:/|$)`),
	},
	{
		// VariantThumbWrap — caption-h3 title flavour.
		ID:        "xsensual",
		SiteBase:  "https://x-sensual.com",
		SiteName:  "X Sensual",
		Variant:   VariantThumbWrap,
		Paginated: true,
		Patterns:  []string{"x-sensual.com/", "x-sensual.com/index.php/main/show_sets2/{N}"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?x-sensual\.com(?:/|$)`),
	},
	{
		// VariantThumbWrap — caption-text title flavour.
		ID:        "privatecastingx",
		SiteBase:  "https://privatecasting-x.com",
		SiteName:  "Private Casting X",
		Variant:   VariantThumbWrap,
		Paginated: true,
		Patterns:  []string{"privatecasting-x.com/", "privatecasting-x.com/index.php/main/show_sets2/{N}"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?privatecasting-x\.com(?:/|$)`),
	},

	// ---- VariantFuckingGlasses ----
	{
		ID:        "fuckingglasses",
		SiteBase:  "https://fuckingglasses.com",
		SiteName:  "Fucking Glasses",
		Variant:   VariantFuckingGlasses,
		Paginated: true,
		Patterns:  []string{"fuckingglasses.com/", "fuckingglasses.com/index.php/main/show_sets2/{N}"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?fuckingglasses\.com(?:/|$)`),
	},

	// ---- VariantBrutalX (reused for massage-x + spypov, which use the
	// same `<div id="N" class="th">` outer + `<h3 class="title_thumb">` (or
	// `<span class="title_thumb">`) + `<span class="duration"><em>`
	// pattern as brutalx) ----
	{
		ID:        "massagex",
		SiteBase:  "https://massage-x.com",
		SiteName:  "Massage X",
		Variant:   VariantBrutalX,
		Paginated: true,
		Patterns:  []string{"massage-x.com/", "massage-x.com/index.php/main/show_sets2/{N}"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?massage-x\.com(?:/|$)`),
	},
	{
		ID:        "spypov",
		SiteBase:  "https://spypov.com",
		SiteName:  "Spy POV",
		Variant:   VariantBrutalX,
		Paginated: true,
		Patterns:  []string{"spypov.com/", "spypov.com/index.php/main/show_sets2/{N}"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?spypov\.com(?:/|$)`),
	},

	// ---- VariantMovieBlock (makehimcuckold cluster, 5 sites) ----
	{
		ID:        "makehimcuckold",
		SiteBase:  "https://makehimcuckold.com",
		SiteName:  "Make Him Cuckold",
		Variant:   VariantMovieBlock,
		Paginated: true,
		Patterns:  []string{"makehimcuckold.com/", "makehimcuckold.com/index.php/main/show_sets2/{N}"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?makehimcuckold\.com(?:/|$)`),
	},
	{
		ID:        "sheisnerdy",
		SiteBase:  "https://sheisnerdy.com",
		SiteName:  "She Is Nerdy",
		Variant:   VariantMovieBlock,
		Paginated: true,
		Patterns:  []string{"sheisnerdy.com/", "sheisnerdy.com/index.php/main/show_sets2/{N}"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?sheisnerdy\.com(?:/|$)`),
	},
	{
		ID:        "momspassions",
		SiteBase:  "https://momspassions.com",
		SiteName:  "Moms Passions",
		Variant:   VariantMovieBlock,
		Paginated: true,
		Patterns:  []string{"momspassions.com/", "momspassions.com/index.php/main/show_sets2/{N}"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?momspassions\.com(?:/|$)`),
	},
	{
		ID:        "trickyourgf",
		SiteBase:  "https://trickyourgf.com",
		SiteName:  "Trick Your GF",
		Variant:   VariantMovieBlock,
		Paginated: true,
		Patterns:  []string{"trickyourgf.com/", "trickyourgf.com/index.php/main/show_sets2/{N}"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?trickyourgf\.com(?:/|$)`),
	},
	{
		ID:        "trickyagent",
		SiteBase:  "https://trickyagent.com",
		SiteName:  "Tricky Agent",
		Variant:   VariantMovieBlock,
		Paginated: true,
		Patterns:  []string{"trickyagent.com/", "trickyagent.com/index.php/main/show_sets2/{N}"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?trickyagent\.com(?:/|$)`),
	},

	// ---- VariantYoungCourtesans ----
	{
		ID:        "youngcourtesans",
		SiteBase:  "https://youngcourtesans.com",
		SiteName:  "Young Courtesans",
		Variant:   VariantYoungCourtesans,
		Paginated: true,
		Patterns:  []string{"youngcourtesans.com/", "youngcourtesans.com/index.php/main/show_sets2/{N}"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?youngcourtesans\.com(?:/|$)`),
	},

	// ---- VariantDebtsex ----
	{
		ID:        "debtsex",
		SiteBase:  "https://debtsex.com",
		SiteName:  "Debt Sex",
		Variant:   VariantDebtsex,
		Paginated: true,
		Patterns:  []string{"debtsex.com/", "debtsex.com/index.php/main/show_sets2/{N}"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?debtsex\.com(?:/|$)`),
	},

	// ---- VariantDisgrace ----
	{
		ID:        "disgracethatbitch",
		SiteBase:  "https://disgracethatbitch.com",
		SiteName:  "Disgrace That Bitch",
		Variant:   VariantDisgrace,
		Paginated: true,
		Patterns:  []string{"disgracethatbitch.com/", "disgracethatbitch.com/index.php/main/show_sets2/{N}"},
		MatchRe:   regexp.MustCompile(`^https?://(?:www\.)?disgracethatbitch\.com(?:/|$)`),
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(New(cfg))
	}
}
