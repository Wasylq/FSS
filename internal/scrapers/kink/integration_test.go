//go:build integration

package kink

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

const liveStudioURL = "https://www.kink.com"

func TestLiveScrape(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 5)
}

func TestLiveScrapeKinkMen(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.kinkmen.com", 5)
}
