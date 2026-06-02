//go:build integration

package extrememoviepass

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/extrememoviepassutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// Sample sites that probed well during platform discovery — full + small
// catalog sites + the parent-redirecting variant.

func TestLiveSexyCuckold(t *testing.T) {
	testutil.RunLiveScrape(t, extrememoviepassutil.New(siteByID(t, "sexycuckold")), "https://www.sexycuckold.com/tour/", 3)
}

func TestLiveSlipperyMassage(t *testing.T) {
	testutil.RunLiveScrape(t, extrememoviepassutil.New(siteByID(t, "slipperymassage")), "https://www.slipperymassage.com/tour/", 3)
}

func TestLiveFlexiDolls(t *testing.T) {
	testutil.RunLiveScrape(t, extrememoviepassutil.New(siteByID(t, "flexidolls")), "https://www.flexidolls.com/tour/", 3)
}

func TestLiveAmateur18(t *testing.T) {
	testutil.RunLiveScrape(t, extrememoviepassutil.New(siteByID(t, "amateur18")), "https://www.amateur18.tv/tour/", 3)
}

func TestLivePornOnStage(t *testing.T) {
	testutil.RunLiveScrape(t, extrememoviepassutil.New(siteByID(t, "pornonstage")), "https://www.pornonstage.com/tour/", 2)
}

func TestLiveSexyCuckoldModelPage(t *testing.T) {
	testutil.RunLiveScrape(t, extrememoviepassutil.New(siteByID(t, "sexycuckold")), "https://www.sexycuckold.com/tour/models/Asya-Murkovski.html", 2)
}

func siteByID(t *testing.T, id string) extrememoviepassutil.SiteConfig {
	t.Helper()
	for _, s := range sites {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("site %q not found in extrememoviepass sites table", id)
	return extrememoviepassutil.SiteConfig{}
}
