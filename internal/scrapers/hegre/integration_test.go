//go:build integration

package hegre

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveHegre(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.hegre.com/", 3)
}
