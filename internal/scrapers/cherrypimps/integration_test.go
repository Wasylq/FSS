//go:build integration

package cherrypimps

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/cherrypimpsutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveCherryPimps(t *testing.T) {
	testutil.RunLiveScrape(t, cherrypimpsutil.New(sites[0]), "https://cherrypimps.com/", 2)
}

func TestLiveCherryPimpsModel(t *testing.T) {
	testutil.RunLiveScrape(t, cherrypimpsutil.New(sites[0]), "https://cherrypimps.com/models/DanaVespoli.html", 2)
}

func TestLiveCherryPimpsCategory(t *testing.T) {
	testutil.RunLiveScrape(t, cherrypimpsutil.New(sites[0]), "https://cherrypimps.com/categories/all-natural.html", 2)
}

func TestLiveCherryPimpsSeries(t *testing.T) {
	testutil.RunLiveScrape(t, cherrypimpsutil.New(sites[0]), "https://cherrypimps.com/series/busted.html", 2)
}

func TestLiveCherryPimpsDVD(t *testing.T) {
	testutil.RunLiveScrape(t, cherrypimpsutil.New(sites[0]), "https://cherrypimps.com/dvds/daddy-complex.html", 2)
}

func TestLiveCherryPimpsDVDListing(t *testing.T) {
	testutil.RunLiveScrape(t, cherrypimpsutil.New(sites[0]), "https://cherrypimps.com/dvds/dvds.html", 2)
}
