//go:build integration

package nvg

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func scraperForID(t *testing.T, id string) *Scraper {
	t.Helper()
	for _, cfg := range sites {
		if cfg.id == id {
			return New(cfg)
		}
	}
	t.Fatalf("no site config for id %q", id)
	return nil
}

func TestLiveNetVideoGirls(t *testing.T) {
	testutil.RunLiveScrape(t, scraperForID(t, "netvideogirls"), "https://netvideogirls.com", 3)
}

func TestLiveCastingCouchHD(t *testing.T) {
	testutil.RunLiveScrape(t, scraperForID(t, "castingcouchhd"), "https://castingcouch-hd.com", 3)
}

func TestLiveNetGirl(t *testing.T) {
	testutil.RunLiveScrape(t, scraperForID(t, "netgirl"), "https://netgirl.com", 3)
}
