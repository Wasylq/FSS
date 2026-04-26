//go:build integration

package yourvids

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrape(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://yourvids.com/creators/rheasweet", 5)
}
