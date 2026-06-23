// Package bronetwork registers the Pinstripe Media Group "Bro Network" gay
// paysites. MASQULIN and Men of Montréal have no independent site — their
// catalogs live as categories on thebronetwork.com. See bronetworkutil.
package bronetwork

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/bronetworkutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []bronetworkutil.SiteConfig{
	{
		ID:       "menatplay",
		Studio:   "MENatPLAY",
		SiteBase: "https://menatplay.com",
		Slug:     "movies",
		Patterns: []string{"menatplay.com", "menatplay.com/categories/movies_{N}_d.html", "menatplay.com/updates/{slug}.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?menatplay\.com`),
	},
	{
		ID:       "amateurgaypov",
		Studio:   "Amateur Gay POV",
		SiteBase: "https://amateurgaypov.com",
		Slug:     "videos",
		Patterns: []string{"amateurgaypov.com", "amateurgaypov.com/categories/videos_{N}_d.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?amateurgaypov\.com`),
	},
	{
		ID:       "thebronetwork",
		Studio:   "The Bro Network",
		SiteBase: "https://thebronetwork.com",
		Slug:     "videos",
		Patterns: []string{"thebronetwork.com", "thebronetwork.com/categories/videos_{N}_d.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?thebronetwork\.com`),
	},
	{
		ID:       "masqulin",
		Studio:   "MASQULIN",
		SiteBase: "https://thebronetwork.com",
		Slug:     "masqulin",
		Patterns: []string{"masqulin.com", "thebronetwork.com/categories/masqulin_{N}_d.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?masqulin\.com`),
	},
	{
		ID:       "menofmontreal",
		Studio:   "Men of Montréal",
		SiteBase: "https://thebronetwork.com",
		Slug:     "men-of-montreal",
		Patterns: []string{"menofmontreal.com", "thebronetwork.com/categories/men-of-montreal_{N}_d.html"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?menofmontreal\.com`),
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(bronetworkutil.New(cfg))
	}
}
