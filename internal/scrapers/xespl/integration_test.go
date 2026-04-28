//go:build integration

package xespl

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

const liveCatalogURL = "https://xes.pl/katalog_filmow,1.html"
const livePerformerURL = "https://xes.pl/aktor,katarzyna-bella-donna,384,1.html"

func TestLiveXesPl(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveCatalogURL)
	testutil.RunLiveScrape(t, New(), liveCatalogURL, 2)
}

func TestLiveXesPlPerformer(t *testing.T) {
	testutil.SkipIfPlaceholder(t, livePerformerURL)
	testutil.RunLiveScrape(t, New(), livePerformerURL, 2)
}
