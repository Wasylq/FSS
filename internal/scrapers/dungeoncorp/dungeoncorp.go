// Package dungeoncorp registers the Dungeon Corp BDSM network brands, all
// served by one PHP app at www.dungeoncorp.com keyed by site code. See
// dungeoncorputil.
package dungeoncorp

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/dungeoncorputil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []dungeoncorputil.SiteConfig{
	{
		ID:       "societysm",
		Studio:   "SocietySM",
		Code:     "SSM",
		Patterns: []string{"societysm.com", "dungeoncorp.com/?page=updates&site=SSM"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?(?:societysm\.com|dungeoncorp\.com/\?page=updates&site=SSM)`),
	},
	{
		ID:       "cumbots",
		Studio:   "Cumbots",
		Code:     "CUM",
		Patterns: []string{"cumbots.com", "dungeoncorp.com/?page=updates&site=CUM"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?(?:cumbots\.com|dungeoncorp\.com/\?page=updates&site=CUM)`),
	},
	{
		ID:       "fuckingdungeon",
		Studio:   "Fucking Dungeon",
		Code:     "FUD",
		Patterns: []string{"dungeoncorp.com/?page=sites&site=FUD", "dungeoncorp.com/?page=updates&site=FUD"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?dungeoncorp\.com/\?page=(?:sites|updates)&site=FUD`),
	},
	{
		ID:       "perfectslave",
		Studio:   "PerfectSlave",
		Code:     "PER",
		Patterns: []string{"perfectslave.com", "dungeoncorp.com/?page=updates&site=PER"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?(?:perfectslave\.com|dungeoncorp\.com/\?page=(?:sites|updates)&site=PER)`),
	},
	{
		ID:       "strictrestraint",
		Studio:   "Strict Restraint",
		Code:     "STR",
		Patterns: []string{"dungeoncorp.com/?page=updates&site=STR"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?dungeoncorp\.com/\?page=(?:sites|updates)&site=STR`),
	},
	{
		ID:       "dungeoncorp",
		Studio:   "DungeonCorp",
		Code:     "DUN",
		Patterns: []string{"dungeoncorp.com", "dungeoncorp.com/?page=updates&site=DUN"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?dungeoncorp\.com/?(?:\?page=(?:sites|updates)&site=DUN)?$`),
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(dungeoncorputil.New(cfg))
	}
}
