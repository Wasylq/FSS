//go:build integration

package kellymadison

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// Fidelity CMS sites.

func TestLivePornFidelity(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("pornfidelity"), "https://www.pornfidelity.com/", 2)
}

func TestLiveTeenFidelity(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("teenfidelity"), "https://www.teenfidelity.com/", 2)
}

func TestLiveKellyMadison(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("kellymadison"), "https://www.kellymadison.com/", 2)
}

// Ultra CMS sites (shared 5kporn.com catalogue, per-prefix filtering).

func TestLive5KPorn(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("5kporn"), "https://www.5kporn.com/", 2)
}

func TestLive5KTeens(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("5kteens"), "https://www.5kteens.com/", 2)
}

func TestLive8KMilfs(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("8kmilfs"), "https://www.8kmilfs.com/", 2)
}

func TestLive8KTeens(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("8kteens"), "https://www.8kteens.com/", 2)
}
