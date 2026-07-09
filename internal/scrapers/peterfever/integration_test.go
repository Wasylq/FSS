//go:build integration

package peterfever

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLive(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.peterfever.com/categories/movies.html", 3)
}
