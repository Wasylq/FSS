// Package adultdoorway registers scrapers for the Adult Doorway / FacialAbuse
// network — 22 sister sites running the Elevated X "Modern" template family
// rooted at /tour/. Parser logic lives in adultdoorwayutil.
//
// Sister site Black Payback uses the older Elevated X "Classic" template
// (flexslider + item-thumb, different pagination form) — it's registered
// separately by the blackpayback package via adultdoorwayclassicutil.
// yournextdoorwhore.net is intentionally not covered: defunct, /tour/ 404s,
// only 3 stashdb scenes.
package adultdoorway

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/adultdoorwayutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []adultdoorwayutil.SiteConfig{
	{
		ID:       "adultdoorway",
		SiteBase: "https://adultdoorway.com",
		Studio:   "Adult Doorway",
		Patterns: []string{
			"adultdoorway.com",
			"adultdoorway.com/tour/categories/{slug}_{page}_d.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:[a-z0-9-]+\.)?adultdoorway\.com`),
	},
	{
		ID:       "amateurthroats",
		SiteBase: "https://amateurthroats.com",
		Studio:   "Amateur Throats",
		Patterns: []string{
			"amateurthroats.com",
			"amateurthroats.com/tour/categories/{slug}_{page}_d.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:[a-z0-9-]+\.)?amateurthroats\.com`),
	},
	{
		ID:       "analrecruiters",
		SiteBase: "https://www.analrecruiters.com",
		Studio:   "Anal Recruiters",
		Patterns: []string{
			"analrecruiters.com",
			"analrecruiters.com/tour/categories/{slug}_{page}_d.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:[a-z0-9-]+\.)?analrecruiters\.com`),
	},
	{
		ID:       "bustyamateurboobs",
		SiteBase: "https://bustyamateurboobs.com",
		Studio:   "Busty Amateur Boobs",
		Patterns: []string{
			"bustyamateurboobs.com",
			"bustyamateurboobs.com/tour/categories/{slug}_{page}_d.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:[a-z0-9-]+\.)?bustyamateurboobs\.com`),
	},
	{
		ID:       "clubamberrayne",
		SiteBase: "https://www.clubamberrayne.com",
		Studio:   "Club Amber Rayne",
		Patterns: []string{
			"clubamberrayne.com",
			"clubamberrayne.com/tour/categories/{slug}_{page}_d.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:[a-z0-9-]+\.)?clubamberrayne\.com`),
	},
	{
		ID:       "ebonycumdumps",
		SiteBase: "https://ebonycumdumps.com",
		Studio:   "Ebony Cum Dumps",
		Patterns: []string{
			"ebonycumdumps.com",
			"ebonycumdumps.com/tour/categories/{slug}_{page}_d.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:[a-z0-9-]+\.)?ebonycumdumps\.com`),
	},
	{
		ID:       "facialabuse",
		SiteBase: "https://facialabuse.com",
		Studio:   "Facial Abuse",
		Patterns: []string{
			"facialabuse.com",
			"facialabuse.com/tour/categories/{slug}_{page}_d.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:[a-z0-9-]+\.)?facialabuse\.com`),
	},
	{
		ID:       "fuckmepov",
		SiteBase: "https://fuckmepov.com",
		Studio:   "Fuck Me POV",
		Patterns: []string{
			"fuckmepov.com",
			"fuckmepov.com/tour/categories/{slug}_{page}_d.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:[a-z0-9-]+\.)?fuckmepov\.com`),
	},
	{
		ID:       "ghettogaggers",
		SiteBase: "https://ghettogaggers.com",
		Studio:   "Ghetto Gaggers",
		Patterns: []string{
			"ghettogaggers.com",
			"ghettogaggers.com/tour/categories/{slug}_{page}_d.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:[a-z0-9-]+\.)?ghettogaggers\.com`),
	},
	{
		ID:       "hardcoredoorway",
		SiteBase: "https://hardcoredoorway.com",
		Studio:   "Hardcore Doorway",
		Patterns: []string{
			"hardcoredoorway.com",
			"hardcoredoorway.com/tour/categories/{slug}_{page}_d.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:[a-z0-9-]+\.)?hardcoredoorway\.com`),
	},
	{
		ID:       "herfirstporn",
		SiteBase: "https://herfirstporn.com",
		Studio:   "Her First Porn",
		Patterns: []string{
			"herfirstporn.com",
			"herfirstporn.com/tour/categories/{slug}_{page}_d.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:[a-z0-9-]+\.)?herfirstporn\.com`),
	},
	{
		ID:       "hugerubberdicks",
		SiteBase: "https://hugerubberdicks.com",
		Studio:   "Huge Rubber Dicks",
		Patterns: []string{
			"hugerubberdicks.com",
			"hugerubberdicks.com/tour/categories/{slug}_{page}_d.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:[a-z0-9-]+\.)?hugerubberdicks\.com`),
	},
	{
		ID:       "joethepervert",
		SiteBase: "https://joethepervert.com",
		Studio:   "Joe the Pervert",
		Patterns: []string{
			"joethepervert.com",
			"joethepervert.com/tour/categories/{slug}_{page}_d.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:[a-z0-9-]+\.)?joethepervert\.com`),
	},
	{
		ID:       "latinaabuse",
		SiteBase: "https://latinaabuse.com",
		Studio:   "Latina Abuse",
		Patterns: []string{
			"latinaabuse.com",
			"latinaabuse.com/tour/categories/{slug}_{page}_d.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:[a-z0-9-]+\.)?latinaabuse\.com`),
	},
	{
		ID:       "monstercockmadness",
		SiteBase: "https://monstercockmadness.com",
		Studio:   "Monster Cock Madness",
		Patterns: []string{
			"monstercockmadness.com",
			"monstercockmadness.com/tour/categories/{slug}_{page}_d.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:[a-z0-9-]+\.)?monstercockmadness\.com`),
	},
	{
		ID:       "nastylittlefacials",
		SiteBase: "https://nastylittlefacials.com",
		Studio:   "Nasty Little Facials",
		Patterns: []string{
			"nastylittlefacials.com",
			"nastylittlefacials.com/tour/categories/{slug}_{page}_d.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:[a-z0-9-]+\.)?nastylittlefacials\.com`),
	},
	{
		ID:       "pinkkittygirls",
		SiteBase: "https://pinkkittygirls.com",
		Studio:   "Pink Kitty Girls",
		Patterns: []string{
			"pinkkittygirls.com",
			"pinkkittygirls.com/tour/categories/{slug}_{page}_d.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:[a-z0-9-]+\.)?pinkkittygirls\.com`),
	},
	{
		ID:       "povhotel",
		SiteBase: "https://povhotel.com",
		Studio:   "POV Hotel",
		Patterns: []string{
			"povhotel.com",
			"povhotel.com/tour/categories/{slug}_{page}_d.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:[a-z0-9-]+\.)?povhotel\.(?:com|net)`),
	},
	{
		ID:       "sexysuckjobs",
		SiteBase: "https://sexysuckjobs.com",
		Studio:   "Sexy Suck Jobs",
		Patterns: []string{
			"sexysuckjobs.com",
			"sexysuckjobs.com/tour/categories/{slug}_{page}_d.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:[a-z0-9-]+\.)?sexysuckjobs\.com`),
	},
	{
		ID:       "spermsuckers",
		SiteBase: "https://spermsuckers.com",
		Studio:   "Sperm Suckers",
		Patterns: []string{
			"spermsuckers.com",
			"spermsuckers.com/tour/categories/{slug}_{page}_d.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:[a-z0-9-]+\.)?spermsuckers\.com`),
	},
	{
		ID:       "thehandjobsite",
		SiteBase: "https://thehandjobsite.com",
		Studio:   "The Handjob Site",
		Patterns: []string{
			"thehandjobsite.com",
			"thehandjobsite.com/tour/categories/{slug}_{page}_d.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:[a-z0-9-]+\.)?thehandjobsite\.com`),
	},
	{
		ID:       "thepantyhosesite",
		SiteBase: "https://thepantyhosesite.com",
		Studio:   "The Pantyhose Site",
		Patterns: []string{
			"thepantyhosesite.com",
			"thepantyhosesite.com/tour/categories/{slug}_{page}_d.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:[a-z0-9-]+\.)?thepantyhosesite\.com`),
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(adultdoorwayutil.New(cfg))
	}
}
