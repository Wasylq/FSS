//go:build integration

package tmw

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/internal/scrapers/tmwutil"
)

func TestLiveScrapeAll(t *testing.T) {
	for _, cfg := range sites {
		t.Run(cfg.SiteID, func(t *testing.T) {
			t.Parallel()
			s := tmwutil.NewScraper(cfg)
			testutil.RunLiveScrape(t, s, "https://"+cfg.Domain+"/categories/movies.html", 2)
		})
	}
}
