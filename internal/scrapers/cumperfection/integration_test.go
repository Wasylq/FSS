//go:build integration

package cumperfection

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveCumPerfection(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.cumperfection.com/categories/movies.html", 3)
}
