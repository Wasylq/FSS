//go:build integration

package czechvr

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveCzechVRNetwork(t *testing.T) {
	s := newSiteScraper(sites[0])
	testutil.RunLiveScrape(t, s, "https://www.czechvrnetwork.com/", 2)
}

func TestLiveCzechVR(t *testing.T) {
	s := newSiteScraper(sites[1])
	testutil.RunLiveScrape(t, s, "https://www.czechvr.com/", 2)
}

func TestLiveCzechVRModel(t *testing.T) {
	s := newSiteScraper(sites[0])
	testutil.RunLiveScrape(t, s, "https://www.czechvrnetwork.com/model-tina-kay", 2)
}
