//go:build integration

package cockyboys

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveCockyBoys(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://cockyboys.com/categories/movies_1_d.html", 3)
}
