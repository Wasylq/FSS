//go:build integration

package scoregroup

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveScrapeAll(t *testing.T) {
	for _, cfg := range sites {
		t.Run(cfg.SiteID, func(t *testing.T) {
			t.Parallel()
			s := newScraper(cfg)
			testutil.RunLiveScrape(t, s, cfg.SiteBase+"/", 2)
		})
	}
}
