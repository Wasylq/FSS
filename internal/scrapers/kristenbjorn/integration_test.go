//go:build integration

package kristenbjorn

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveKristenBjorn(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.kristenbjorn.com", 3)
}
