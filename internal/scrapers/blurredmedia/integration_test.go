//go:build integration

package blurredmedia

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveHotGuysFuck(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("hotguysfuck"), "https://hotguysfuck.com/", 2)
}

func TestLiveBiGuysFuck(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("biguysfuck"), "https://biguysfuck.com/", 2)
}

func TestLiveGayHoopla(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("gayhoopla"), "https://gayhoopla.com/", 2)
}

func TestLiveSugarDaddyPorn(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("sugardaddyporn"), "https://sugardaddyporn.com/", 2)
}
