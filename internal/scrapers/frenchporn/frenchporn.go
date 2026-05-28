// Package frenchporn registers scrapers for every PornSiteManager-backed
// site in the Frenchporn network — 23 brands (parent portal + 22 children)
// sharing one CMS. Crunchboy is also part of the network but has its own
// standalone scraper (`internal/scrapers/crunchboy`) that predates this util.
// Parser logic lives in `psmutil`.
package frenchporn

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/psmutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []psmutil.SiteConfig{
	// Network portal — aggregates content from all sister sites.
	{
		ID:       "frenchporn",
		SiteBase: "https://www.frenchporn.fr",
		Studio:   "Frenchporn",
		Patterns: []string{
			"frenchporn.fr",
			"frenchporn.fr/en/videos",
			"frenchporn.fr/en/videos/{category}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?frenchporn\.fr`),
	},
	{
		ID:       "alphamales",
		SiteBase: "https://www.alphamales.com",
		Studio:   "AlphaMales",
		Patterns: []string{
			"alphamales.com",
			"alphamales.com/en/videos",
			"alphamales.com/en/videos/{category}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?alphamales\.com`),
	},
	{
		ID:       "andolinixxl",
		SiteBase: "https://www.andolinixxl.com",
		Studio:   "AndoliniXXL",
		Patterns: []string{
			"andolinixxl.com",
			"andolinixxl.com/en/videos",
			"andolinixxl.com/en/videos/{category}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?andolinixxl\.com`),
	},
	{
		ID:       "attackboys",
		SiteBase: "https://www.attackboys.com",
		Studio:   "AttackBoys",
		Patterns: []string{
			"attackboys.com",
			"attackboys.com/en/videos",
			"attackboys.com/en/videos/{category}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?attackboys\.com`),
	},
	{
		ID:       "berryboys",
		SiteBase: "https://www.berryboys.fr",
		Studio:   "Berryboys",
		Patterns: []string{
			"berryboys.fr",
			"berryboys.fr/en/videos",
			"berryboys.fr/en/videos/{category}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?berryboys\.fr`),
	},
	{
		ID:       "bolatino",
		SiteBase: "https://www.bolatino.com",
		Studio:   "BoLatino",
		Patterns: []string{
			"bolatino.com",
			"bolatino.com/en/videos",
			"bolatino.com/en/videos/{category}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?bolatino\.com`),
	},
	{
		ID:       "bravofucker",
		SiteBase: "https://www.bravofucker.com",
		Studio:   "Bravo Fucker",
		Patterns: []string{
			"bravofucker.com",
			"bravofucker.com/en/videos",
			"bravofucker.com/en/videos/{category}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?bravofucker\.com`),
	},
	{
		ID:       "bretttyler",
		SiteBase: "https://www.brett-tyler.com",
		Studio:   "Brett Tyler",
		Patterns: []string{
			"brett-tyler.com",
			"brett-tyler.com/en/videos",
			"brett-tyler.com/en/videos/{category}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?brett-tyler\.com`),
	},
	{
		ID:       "bulldogxxx",
		SiteBase: "https://www.bulldogxxx.com",
		Studio:   "Bulldog XXX",
		Patterns: []string{
			"bulldogxxx.com",
			"bulldogxxx.com/en/videos",
			"bulldogxxx.com/en/videos/{category}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?bulldogxxx\.com`),
	},
	{
		ID:       "cadinot",
		SiteBase: "https://www.cadinot.fr",
		Studio:   "Cadinot",
		Patterns: []string{
			"cadinot.fr",
			"cadinot.fr/en/videos",
			"cadinot.fr/en/videos/{category}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?cadinot\.fr`),
	},
	{
		ID:       "cazzofilm",
		SiteBase: "https://www.cazzofilm.com",
		Studio:   "Cazzofilm",
		Patterns: []string{
			"cazzofilm.com",
			"cazzofilm.com/en/videos",
			"cazzofilm.com/en/videos/{category}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?cazzofilm\.com`),
	},
	{
		ID:       "citebeur",
		SiteBase: "https://www.citebeur.com",
		Studio:   "Citebeur",
		Patterns: []string{
			"citebeur.com",
			"citebeur.com/en/videos",
			"citebeur.com/en/videos/{category}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?citebeur\.com`),
	},
	{
		ID:       "clairprod",
		SiteBase: "https://www.clairprod.com",
		Studio:   "Clairprod",
		Patterns: []string{
			"clairprod.com",
			"clairprod.com/en/videos",
			"clairprod.com/en/videos/{category}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?clairprod\.com`),
	},
	{
		ID:       "cocksuckerprod",
		SiteBase: "https://www.cocksuckerprod.com",
		Studio:   "Cocksucker",
		Patterns: []string{
			"cocksuckerprod.com",
			"cocksuckerprod.com/en/videos",
			"cocksuckerprod.com/en/videos/{category}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?cocksuckerprod\.com`),
	},
	{
		ID:       "eurocreme",
		SiteBase: "https://www.eurocreme.com",
		Studio:   "Eurocreme",
		Patterns: []string{
			"eurocreme.com",
			"eurocreme.com/en/videos",
			"eurocreme.com/en/videos/{category}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?eurocreme\.com`),
	},
	{
		ID:       "gayfrenchkiss",
		SiteBase: "https://www.gayfrenchkiss.fr",
		Studio:   "Gay French Kiss",
		Patterns: []string{
			"gayfrenchkiss.fr",
			"gayfrenchkiss.com",
			"gayfrenchkiss.fr/en/videos",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?gayfrenchkiss\.(?:fr|com)`),
	},
	{
		ID:       "hardkinks",
		SiteBase: "https://www.hardkinks.com",
		Studio:   "Hard Kinks",
		Patterns: []string{
			"hardkinks.com",
			"hardkinks.com/en/videos",
			"hardkinks.com/en/videos/{category}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?hardkinks\.com`),
	},
	{
		ID:       "harlemsex",
		SiteBase: "https://www.harlemsex.com",
		Studio:   "Harlemsex",
		Patterns: []string{
			"harlemsex.com",
			"harlemsex.com/en/videos",
			"harlemsex.com/en/videos/{category}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?harlemsex\.com`),
	},
	{
		ID:       "mikaayden",
		SiteBase: "https://www.mika-ayden.com",
		Studio:   "Mika Ayden",
		Patterns: []string{
			"mika-ayden.com",
			"mika-ayden.com/en/videos",
			"mika-ayden.com/en/videos/{category}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?mika-ayden\.com`),
	},
	{
		ID:       "militarygayxxx",
		SiteBase: "https://www.militarygayxxx.com",
		Studio:   "Military Gay XXX",
		Patterns: []string{
			"militarygayxxx.com",
			"militarygayxxx.com/en/videos",
			"militarygayxxx.com/en/videos/{category}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?militarygayxxx\.com`),
	},
	{
		ID:       "rawboys",
		SiteBase: "https://www.rawboys.com",
		Studio:   "Raw Boys",
		Patterns: []string{
			"rawboys.com",
			"rawboys.com/en/videos",
			"rawboys.com/en/videos/{category}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?rawboys\.com`),
	},
	{
		ID:       "rawfuck",
		SiteBase: "https://www.rawfuck.com",
		Studio:   "Rawfuck.com",
		Patterns: []string{
			"rawfuck.com",
			"rawfuck.com/en/videos",
			"rawfuck.com/en/videos/{category}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?rawfuck\.com`),
	},
	{
		ID:       "universblack",
		SiteBase: "https://www.universblack.com",
		Studio:   "Univers Black",
		Patterns: []string{
			"universblack.com",
			"universblack.com/en/videos",
			"universblack.com/en/videos/{category}",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?universblack\.com`),
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(psmutil.New(cfg))
	}
}
