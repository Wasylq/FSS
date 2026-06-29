//go:build integration

package strokies

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/strokiesutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveStrokies(t *testing.T) {
	testutil.RunLiveScrape(t, strokiesutil.New(sites[0]), "https://strokies.com/", 3)
}

func TestLiveTugCasting(t *testing.T) {
	testutil.RunLiveScrape(t, strokiesutil.New(sites[1]), "https://tugcasting.com/", 3)
}
