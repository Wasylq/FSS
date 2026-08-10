//go:build integration

package woodmanfilms

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveWoodmanFilms(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.woodmanfilms.com/", 2)
}

// The pornstar mode dispatches through pornstarRe to runPornstar, a separate
// parser from the main listing. Pornstar URLs are only reachable from a scene
// page, not the homepage, so this mode had no live coverage at all.
func TestLiveWoodmanFilmsPornstar(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.woodmanfilms.com/pornstar/angelina_sweet_2136", 2)
}
