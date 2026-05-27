//go:build integration

package prestige

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLivePrestige(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.prestige-av.com/goods", 2)
}
