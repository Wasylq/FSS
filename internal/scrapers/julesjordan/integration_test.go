//go:build integration

package julesjordan

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/julesjordanutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrapeAll(t *testing.T) {
	for _, cfg := range sites {
		t.Run(cfg.SiteID, func(t *testing.T) {
			t.Parallel()
			s := julesjordanutil.New(cfg)
			testutil.RunLiveScrape(t, s, "https://www."+cfg.Domain+"/trial/categories/movies.html", 2)
		})
	}
}
