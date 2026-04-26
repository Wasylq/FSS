//go:build integration

package kink

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

const liveStudioURL = "https://www.kink.com"

func TestLiveScrape(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 5)
}
