//go:build integration

package ladyfyre

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveLadyFyre(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.ladyfyre.com/tour/categories/movies.html", 2)
}
