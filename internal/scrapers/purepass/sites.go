package purepass

import (
	"regexp"

	"github.com/Wasylq/FSS/scraper"
)

var sites = []SiteConfig{
	{
		ID:       "purecfnm",
		SiteBase: "https://www.purecfnm.com",
		SiteName: "Pure CFNM",
		Patterns: []string{
			"purecfnm.com",
			"purecfnm.com/categories/{slug}_{page}_d.html",
			"purecfnm.com/models/{slug}.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?purecfnm\.com(?:/|$)`),
	},
	{
		ID:       "amateurcfnm",
		SiteBase: "https://www.amateurcfnm.com",
		SiteName: "Amateur CFNM",
		Patterns: []string{
			"amateurcfnm.com",
			"amateurcfnm.com/categories/{slug}_{page}_d.html",
			"amateurcfnm.com/models/{slug}.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?amateurcfnm\.com(?:/|$)`),
	},
	{
		ID:       "cfnmgames",
		SiteBase: "https://www.cfnmgames.com",
		SiteName: "CFNM Games",
		Patterns: []string{
			"cfnmgames.com",
			"cfnmgames.com/categories/{slug}_{page}_d.html",
			"cfnmgames.com/models/{slug}.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?cfnmgames\.com(?:/|$)`),
	},
	{
		ID:       "girlsabuseguys",
		SiteBase: "https://www.girlsabuseguys.com",
		SiteName: "Girls Abuse Guys",
		Patterns: []string{
			"girlsabuseguys.com",
			"girlsabuseguys.com/categories/{slug}_{page}_d.html",
			"girlsabuseguys.com/models/{slug}.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?girlsabuseguys\.com(?:/|$)`),
	},
	{
		ID:       "ladyvoyeurs",
		SiteBase: "https://www.ladyvoyeurs.com",
		SiteName: "Lady Voyeurs",
		Patterns: []string{
			"ladyvoyeurs.com",
			"ladyvoyeurs.com/categories/{slug}_{page}_d.html",
			"ladyvoyeurs.com/models/{slug}.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?ladyvoyeurs\.com(?:/|$)`),
	},
	{
		ID:       "littledickclub",
		SiteBase: "https://littledick.club",
		SiteName: "Little Dick Club",
		Patterns: []string{
			"littledick.club",
			"littledick.club/categories/{slug}_{page}_d.html",
			"littledick.club/models/{slug}.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?littledick\.club(?:/|$)`),
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(New(cfg))
	}
}
