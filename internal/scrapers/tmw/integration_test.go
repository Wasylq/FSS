//go:build integration

package tmw

import (
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/internal/scrapers/tmwutil"
)

func TestLiveScrapeAll(t *testing.T) {
	for i, cfg := range sites {
		if i > 0 {
			time.Sleep(500 * time.Millisecond)
		}
		t.Run(cfg.SiteID, func(t *testing.T) {
			s := tmwutil.NewScraper(cfg)
			var url string
			if cfg.Hub {
				url = "https://teenmegaworld.net/categories/" + cfg.Slug + "_1_d.html"
			} else {
				url = "https://" + cfg.Domain + "/categories/movies.html"
			}
			testutil.RunLiveScrape(t, s, url, 2)
		})
	}
}
