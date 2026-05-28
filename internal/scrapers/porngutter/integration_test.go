//go:build integration

package porngutter

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// TestLivePornGutter scrapes the global /updates/ catalogue via the parent
// brand. All 19 sister domains hit the same backend so one live test
// validates the shared parser.
func TestLivePornGutter(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://porngutter.com/updates/", 4)
}

// TestLivePornGutterBonusSite exercises the per-site filtered listing path,
// which has its own card-count (30/page vs 12/page) and pagination block.
func TestLivePornGutterBonusSite(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://porngutter.com/bonus_site/smut-merchants/", 4)
}
