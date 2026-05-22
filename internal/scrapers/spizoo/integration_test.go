//go:build integration

package spizoo

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/spizooutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

const liveStudioURL = "https://www.spizoo.com/"

func TestLiveSpizoo(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	s := spizooutil.New(spizooutil.SiteConfig{
		SiteID:     "spizoo",
		Domain:     "spizoo.com",
		StudioName: "Spizoo",
	})
	testutil.RunLiveScrape(t, s, liveStudioURL, 2)
}
