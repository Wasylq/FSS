//go:build integration

package fpn

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveAnalized(t *testing.T) {
	s := newSiteScraper(sites[4]) // analized
	testutil.RunLiveScrape(t, s, "https://analized.com/", 2)
}

func TestLiveBadDaddyPOV(t *testing.T) {
	s := newSiteScraper(sites[6]) // baddaddypov
	testutil.RunLiveScrape(t, s, "https://baddaddypov.com/", 2)
}

func TestLiveFullPornNetwork(t *testing.T) {
	s := newSiteScraper(sites[0]) // fullpornnetwork
	testutil.RunLiveScrape(t, s, "https://fullpornnetwork.com/", 2)
}
