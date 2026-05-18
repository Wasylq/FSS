//go:build integration

package onepassforallsites

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLive1PassForAllSites(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://1passforallsites.com/", 3)
}

func TestLiveOldGoesYoung(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://oldgoesyoung.com/", 2)
}
