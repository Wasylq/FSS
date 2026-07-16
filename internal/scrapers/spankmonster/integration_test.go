//go:build integration

package spankmonster

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveSpankMonster(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.spankmonster.com/spank-monster-updates.html", 3)
}
