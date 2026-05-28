//go:build integration

package frenchporn

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/psmutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// Sample the parent + a handful of representative children to verify the
// JSON-LD pipeline holds across the network.

func TestLiveFrenchpornParent(t *testing.T) {
	testutil.RunLiveScrape(t, psmutil.New(siteByID(t, "frenchporn")), "https://www.frenchporn.fr/en/videos", 3)
}

func TestLiveCitebeur(t *testing.T) {
	testutil.RunLiveScrape(t, psmutil.New(siteByID(t, "citebeur")), "https://www.citebeur.com/en/videos", 3)
}

func TestLiveEurocreme(t *testing.T) {
	testutil.RunLiveScrape(t, psmutil.New(siteByID(t, "eurocreme")), "https://www.eurocreme.com/en/videos", 3)
}

func TestLiveHardKinks(t *testing.T) {
	testutil.RunLiveScrape(t, psmutil.New(siteByID(t, "hardkinks")), "https://www.hardkinks.com/en/videos", 3)
}

func TestLiveAlphaMales(t *testing.T) {
	testutil.RunLiveScrape(t, psmutil.New(siteByID(t, "alphamales")), "https://www.alphamales.com/en/videos", 3)
}

func TestLiveCitebeurCategory(t *testing.T) {
	testutil.RunLiveScrape(t, psmutil.New(siteByID(t, "citebeur")), "https://www.citebeur.com/en/videos/arab-french", 3)
}

// Sanity-check one of the less-trafficked sites — confirms the JSON-LD
// pipeline holds even when a child has a small catalog.
func TestLiveAttackBoys(t *testing.T) {
	testutil.RunLiveScrape(t, psmutil.New(siteByID(t, "attackboys")), "https://www.attackboys.com/en/videos", 2)
}

func siteByID(t *testing.T, id string) psmutil.SiteConfig {
	t.Helper()
	for _, s := range sites {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("site %q not found in frenchporn sites table", id)
	return psmutil.SiteConfig{}
}
