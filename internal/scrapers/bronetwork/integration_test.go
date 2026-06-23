//go:build integration

package bronetwork

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/bronetworkutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveMenAtPlay(t *testing.T) {
	testutil.RunLiveScrape(t, bronetworkutil.New(sites[0]), "https://menatplay.com/", 3)
}

func TestLiveBroNetwork(t *testing.T) {
	testutil.RunLiveScrape(t, bronetworkutil.New(sites[2]), "https://thebronetwork.com/", 3)
}

func TestLiveMasqulin(t *testing.T) {
	testutil.RunLiveScrape(t, bronetworkutil.New(sites[3]), "https://masqulin.com/", 3)
}
