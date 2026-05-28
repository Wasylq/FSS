//go:build integration

package adultdoorway

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/adultdoorwayutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// One representative site per template variant — facialabuse uses the
// videothumb cards, povhotel uses the plain-image variant.

func TestLiveFacialAbuse(t *testing.T) {
	testutil.RunLiveScrape(t, adultdoorwayutil.New(siteByID(t, "facialabuse")), "https://facialabuse.com/", 2)
}

func TestLiveFacialAbuseCategory(t *testing.T) {
	testutil.RunLiveScrape(t, adultdoorwayutil.New(siteByID(t, "facialabuse")), "https://facialabuse.com/tour/categories/Anal_1_d.html", 2)
}

func TestLivePOVHotel(t *testing.T) {
	testutil.RunLiveScrape(t, adultdoorwayutil.New(siteByID(t, "povhotel")), "https://povhotel.com/", 2)
}

func TestLiveGhettoGaggers(t *testing.T) {
	testutil.RunLiveScrape(t, adultdoorwayutil.New(siteByID(t, "ghettogaggers")), "https://ghettogaggers.com/", 2)
}

func TestLiveAdultDoorwayParent(t *testing.T) {
	testutil.RunLiveScrape(t, adultdoorwayutil.New(siteByID(t, "adultdoorway")), "https://adultdoorway.com/", 2)
}

func siteByID(t *testing.T, id string) adultdoorwayutil.SiteConfig {
	t.Helper()
	for _, s := range sites {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("site %q not found in adultdoorway sites table", id)
	return adultdoorwayutil.SiteConfig{}
}
