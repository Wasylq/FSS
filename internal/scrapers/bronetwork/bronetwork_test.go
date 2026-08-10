package bronetwork

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
	"github.com/Anastylosis/FSS/scraper"
)

// bronetwork is a network-style table: three rows share the network's SiteBase
// (thebronetwork.com) and carry MatchRe for their own vanity domains, so only
// the universal checks apply — CheckSiteTableDomains would flag that shared base
// as a copy-paste slip, which it is not.
func TestSiteTableIntegrity(t *testing.T) {
	rows := make([]testutil.SiteRow, 0, len(sites))
	for _, c := range sites {
		rows = append(rows, testutil.SiteRow{
			ID: c.ID, Base: c.SiteBase, Studio: c.Studio,
			Patterns: c.Patterns, MatchRe: c.MatchRe,
		})
	}
	testutil.CheckSiteTable(t, rows)
}

// Sub-studio listing URLs live on the network's own domain, so the network's
// catch-all regex matches them too. scraper.ForURL returns the *first*
// registered match, which means the sub-studios must come before the catch-all
// in `sites` — get that wrong and pasting a masqulin URL silently scrapes the
// whole Bro Network and stores it under the masqulin URL's key.
//
// This pins the routing rather than the ordering, so it stays true however the
// overlap is resolved later.
func TestSubStudioURLsRouteToTheirOwnScraper(t *testing.T) {
	cases := []struct{ url, want string }{
		{"https://thebronetwork.com/categories/masqulin_1_d.html", "masqulin"},
		{"https://thebronetwork.com/categories/masqulin.html", "masqulin"},
		{"https://masqulin.com/", "masqulin"},
		{"https://thebronetwork.com/categories/men-of-montreal_1_d.html", "menofmontreal"},
		{"https://menofmontreal.com/", "menofmontreal"},
		// The network keeps everything that is not a sub-studio.
		{"https://thebronetwork.com/", "thebronetwork"},
		{"https://thebronetwork.com/categories/videos_1_d.html", "thebronetwork"},
		// Independent sites are unaffected.
		{"https://menatplay.com/", "menatplay"},
		{"https://amateurgaypov.com/", "amateurgaypov"},
	}
	for _, c := range cases {
		got, err := scraper.ForURL(c.url)
		if err != nil {
			t.Errorf("ForURL(%q): %v", c.url, err)
			continue
		}
		if got.ID() != c.want {
			t.Errorf("ForURL(%q) = %q, want %q", c.url, got.ID(), c.want)
		}
	}
}
