//go:build integration

package swallowsalon

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveSwallowSalon(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.swallowsalon.com/categories/movies_1_d.html", 3)
}
