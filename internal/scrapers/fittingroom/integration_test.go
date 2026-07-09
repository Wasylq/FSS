//go:build integration

package fittingroom

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveFittingRoom(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.fitting-room.com/videos_list.php", 3)
}
