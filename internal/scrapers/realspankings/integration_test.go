//go:build integration

package realspankings

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/realspankingsutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrapeAll(t *testing.T) {
	for _, cfg := range sites {
		t.Run(cfg.SiteID, func(t *testing.T) {
			t.Parallel()
			s := realspankingsutil.NewScraper(cfg)
			testutil.RunLiveScrape(t, s, "https://"+cfg.Domain, 2)
		})
	}
}
