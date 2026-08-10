//go:build integration

package dickdrainers

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveScrape(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://dickdrainers.com/tour/categories/movies/1/latest/", 3)
}
