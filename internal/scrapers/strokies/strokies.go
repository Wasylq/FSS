// Package strokies registers the Mack Kensington network sites (modern
// ElevatedX "v-thumb" tour template). See strokiesutil.
package strokies

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/strokiesutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []strokiesutil.SiteConfig{
	{
		ID:       "strokies",
		Studio:   "Strokies",
		SiteBase: "https://strokies.com",
		Patterns: []string{"strokies.com", "strokies.com/page{N}/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?strokies\.com`),
	},
	{
		ID:       "tugcasting",
		Studio:   "TugCasting",
		SiteBase: "https://tugcasting.com",
		Patterns: []string{"tugcasting.com", "tugcasting.com/page{N}/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?tugcasting\.com`),
	},
	{
		ID:       "publichandjobs",
		Studio:   "Public Handjobs",
		SiteBase: "https://publichandjobs.com",
		Patterns: []string{"publichandjobs.com", "publichandjobs.com/page{N}/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?publichandjobs\.com`),
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(strokiesutil.New(cfg))
	}
}
