// Package bigbootytgirls scrapes bigbootytgirls.com, the one Trans500 child
// with a live catalogue on its own domain.
//
// It runs the ElevatedX "updateItem" tour from the document root —
// `/categories/updates_{N}_d.html` listings, `/updates/{slug}.html` detail
// pages, `<h5>` titles, `tour_update_models` credits and `availdate` dates —
// which is exactly what darkreachupdateitemutil parses. That util is named for
// the network it was written against, but the template is a vendor one and is
// not Darkreach-specific (it already covers hammerboys.tv and terrorxxx), so
// this reuses it rather than restating the parser.
//
// Two sibling domains, ikillitts.com and tsgirlfriendexperience.com, redirect
// here; their catalogues are reachable as categories of the network tour and
// are covered by the trans500 package.
package bigbootytgirls

import (
	"regexp"

	"github.com/Anastylosis/FSS/internal/scrapers/darkreachupdateitemutil"
	"github.com/Anastylosis/FSS/scraper"
)

func New() *darkreachupdateitemutil.Scraper {
	return darkreachupdateitemutil.New(darkreachupdateitemutil.SiteConfig{
		ID:       "bigbootytgirls",
		SiteBase: "https://bigbootytgirls.com",
		Studio:   "Big Booty TGirls",
		Patterns: []string{
			"bigbootytgirls.com",
			"bigbootytgirls.com/categories/updates_{N}_d.html",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?bigbootytgirls\.com(?:/|$)`),
	})
}

func init() { scraper.Register(New()) }
