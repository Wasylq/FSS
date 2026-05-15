//go:build integration

package wankz

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/internal/scrapers/wankzutil"
)

func TestLiveScrape(t *testing.T) {
	for _, cfg := range sites {
		t.Run(cfg.SiteID, func(t *testing.T) {
			t.Parallel()
			s := &siteScraper{
				wankz: wankzutil.NewScraper(wankzutil.SiteConfig{
					SiteID:     cfg.SiteID,
					SiteBase:   "https://www." + cfg.Domain,
					StudioName: cfg.StudioName,
				}),
				config: cfg,
			}
			testutil.RunLiveScrape(t, s, "https://www."+cfg.Domain+"/", 3)
		})
	}
}
