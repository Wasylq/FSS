//go:build integration

package youthlust

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveYouthLust(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://youthlust.club/", 3)
}
