// Package himeros scrapes himeros.tv.
//
// It runs the Adult Doorway variant of the Elevated X "Modern" template —
// `item-update` cards under `/tour/categories/movies_{N}_d.html`, bare-slug
// `/tour/trailers/{slug}.html` detail pages — so it reuses that util rather
// than restating the parser. The template is a vendor one; the Adult Doorway
// name is only where FSS first met it.
package himeros

import (
	"regexp"

	"github.com/Anastylosis/FSS/internal/scrapers/adultdoorwayutil"
	"github.com/Anastylosis/FSS/scraper"
)

func New() *adultdoorwayutil.Scraper {
	return adultdoorwayutil.New(adultdoorwayutil.SiteConfig{
		ID:       "himeros",
		SiteBase: "https://himeros.tv",
		Studio:   "Himeros.TV",
		Patterns: []string{
			"himeros.tv",
			"himeros.tv/tour/categories/movies_{N}_d.html",
			"himeros.tv/tour/trailers/{slug}.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?himeros\.tv(?:/|$)`),
	})
}

func init() { scraper.Register(New()) }
