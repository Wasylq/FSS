//go:build integration

package naughtyamerica

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// liveStudioURL — pick a real performer/category URL.
// Pattern: https://www.naughtyamerica.com/<various>
const liveStudioURL = "https://www.naughtyamerica.com/pornstar/cherie-deville"

func TestLiveNaughtyAmerica(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
