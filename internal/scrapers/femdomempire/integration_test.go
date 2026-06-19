//go:build integration

package femdomempire

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLive(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://femdomempire.com/tour/categories/movies/1/latest/", 2)
}
