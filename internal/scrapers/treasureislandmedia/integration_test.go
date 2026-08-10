//go:build integration

package treasureislandmedia

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveTreasureIslandMedia(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://treasureislandmedia.com/scenes?channel=All&page=1", 3)
}
