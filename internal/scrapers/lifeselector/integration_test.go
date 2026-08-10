//go:build integration

package lifeselector

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveLifeSelector(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://lifeselector.com/games", 3)
}
