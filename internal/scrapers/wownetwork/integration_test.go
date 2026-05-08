//go:build integration

package wownetwork

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/internal/scrapers/wownetworkutil"
)

func TestLiveWowGirls(t *testing.T) {
	const url = "https://wowgirls.com"
	testutil.SkipIfPlaceholder(t, url)
	s := wownetworkutil.New(sites[0])
	testutil.RunLiveScrape(t, s, url, 2)
}

func TestLiveUltraFilms(t *testing.T) {
	const url = "https://ultrafilms.com"
	testutil.SkipIfPlaceholder(t, url)
	s := wownetworkutil.New(sites[1])
	testutil.RunLiveScrape(t, s, url, 2)
}
