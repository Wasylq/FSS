//go:build integration

package meanworld

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveMeanWorld(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://megasite.meanworld.com/categories/movies_1_d.html", 3)
}
