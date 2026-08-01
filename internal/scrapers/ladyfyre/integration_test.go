//go:build integration

package ladyfyre

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveLadyFyre(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.ladyfyre.com/tour/categories/movies.html", 2)
}

// A /models/ URL takes runSinglePage instead of runPaginated — one fetch, no
// pagination, and a different parse. Untested until now.
func TestLiveLadyFyreModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.ladyfyre.com/tour/models/LadyOliviaFyre.html", 2)
}
