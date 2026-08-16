//go:build integration

package hungyoungbrit

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveHungYoungBrit(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.hungyoungbrit.com/tour/", 2)
}

// A single scene URL skips the listing walk.
func TestLiveHungYoungBritScene(t *testing.T) {
	testutil.RunLiveScrape(t, New(),
		"https://www.hungyoungbrit.com/tour/updates/Alexis-Gets-Pounded-in-Anonymous-Sauna-Steamer-Quay-Torquay.html", 1)
}
