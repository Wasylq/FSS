//go:build integration

package guysinsweatpants

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveGuysInSweatpants(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://guysinsweatpants.com/tour/categories/movies.html", 3)
}
