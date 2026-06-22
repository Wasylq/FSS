//go:build integration

package malibumedia

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveXArt(t *testing.T) {
	const u = "https://www.x-art.com/"
	testutil.SkipIfPlaceholder(t, u)
	testutil.RunLiveScrape(t, newFor("x-art"), u, 2)
}

func TestLiveColette(t *testing.T) {
	const u = "https://www.colettevideos.com/"
	testutil.SkipIfPlaceholder(t, u)
	testutil.RunLiveScrape(t, newFor("colettevideos"), u, 2)
}
