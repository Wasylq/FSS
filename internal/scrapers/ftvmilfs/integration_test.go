//go:build integration

package ftvmilfs

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

const liveURL = "https://ftvmilfs.com/updates.html"

func TestLiveFTVMilfs(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveURL)
	testutil.RunLiveScrape(t, New(), liveURL, 2)
}
