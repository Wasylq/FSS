//go:build integration

package adultprime

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/adultprimeutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrapeAll(t *testing.T) {
	for _, cfg := range sites {
		t.Run(cfg.SiteID, func(t *testing.T) {
			t.Parallel()
			s := adultprimeutil.New(cfg)
			testutil.RunLiveScrape(t, s, "https://adultprime.com/studios/studio/"+cfg.Slug, 2)
		})
	}
}

func TestLiveScrapeNicheFilter(t *testing.T) {
	s := adultprimeutil.New(sites[0])
	testutil.RunLiveScrape(t, s, "https://adultprime.com/studios/videos?website="+sites[0].Slug+"&niche=Blowjob", 2)
}
