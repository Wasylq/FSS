//go:build integration

package staxus

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrape(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://staxus.com/trial/category.php?id=50&lang=0&s=d", 3)
}
