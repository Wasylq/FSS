//go:build integration

package swearl

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/swearlutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func scraperForID(t *testing.T, id string) *swearlutil.Scraper {
	t.Helper()
	for _, cfg := range sites {
		if cfg.ID == id {
			return swearlutil.New(cfg)
		}
	}
	t.Fatalf("no site config for id %q", id)
	return nil
}

func TestLiveVRBangers(t *testing.T) {
	testutil.RunLiveScrape(t, scraperForID(t, "vrbangers"), "https://vrbangers.com", 3)
}

func TestLiveVRBTrans(t *testing.T) {
	testutil.RunLiveScrape(t, scraperForID(t, "vrbtrans"), "https://vrbtrans.com", 3)
}
