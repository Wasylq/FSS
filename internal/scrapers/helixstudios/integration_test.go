//go:build integration

package helixstudios

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveHelixStudios(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("helixstudios"), "https://www.helixstudios.net/", 2)
}

func TestLiveHelixEurope(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("helixstudios"),
		"https://www.helixstudios.net/watch-newest-helix-studios-clips-and-scenes.html?series=62682", 2)
}

func TestLive8teenBoy(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("8teenboy"), "https://www.8teenboy.com/", 2)
}

func TestLiveSpankThis(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("spankthis"), "https://www.spankthis.com/", 2)
}
