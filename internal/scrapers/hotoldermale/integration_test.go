//go:build integration

package hotoldermale

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveHotOlderMale(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.hotoldermale.com/", 2)
}

// A category and a performer page render the same cards, so both walk the same
// way over a different path.
func TestLiveHotOlderMaleCategory(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.hotoldermale.com/scenes/category/3-bears", 2)
}

func TestLiveHotOlderMaleProfile(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.hotoldermale.com/profile/609-mack-austin", 2)
}

// A single scene URL skips the walk entirely.
func TestLiveHotOlderMaleScene(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.hotoldermale.com/scene/910-daddy-mack-austin-fucks-muscle-stud-davin", 1)
}
