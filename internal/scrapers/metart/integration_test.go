//go:build integration

package metart

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/metartutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrapeAll(t *testing.T) {
	for _, cfg := range sites {
		t.Run(cfg.SiteID, func(t *testing.T) {
			t.Parallel()
			s := metartutil.New(cfg)
			testutil.RunLiveScrape(t, s, "https://www."+cfg.Domain, 2)
		})
	}
}
