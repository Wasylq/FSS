//go:build integration

package primalfetish

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func TestLiveScrape(t *testing.T) {
	cases := []struct {
		name       string
		url        string
		wantSiteID string
	}{
		{"taboo", "https://primalfetishnetwork.com/paysites/14/primals-taboo-relations/videos/", "primalstaboofamily"},
		{"cosplay", "https://primalfetishnetwork.com/paysites/32/primals-cosplay/videos/", "primalscosplay"},
		{"wrestling", "https://primalfetishnetwork.com/paysites/16/primals-wrestling-sex/videos/", "primalswrestlingsex"},
		{"eroticmassage", "https://primalfetishnetwork.com/paysites/1/erotic-massage-master/videos/", "primaleroticmassage"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, err := scraper.ForURL(tc.url)
			if err != nil {
				t.Fatalf("ForURL(%s): %v", tc.url, err)
			}
			if s.ID() != tc.wantSiteID {
				t.Fatalf("ForURL matched %q, want %q", s.ID(), tc.wantSiteID)
			}
			testutil.RunLiveScrape(t, s, tc.url, 3)
		})
	}
}
