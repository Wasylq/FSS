//go:build integration

package thatfetishgirl

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveThatFetishGirl(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://thatfetishgirl.com/updates/page_1.html", 3)
}
