//go:build integration

package sexmex

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrape(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://sexmex.xxx/tour/updates", 5)
}

func TestLiveScrapeModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://sexmex.xxx/tour/models/NickyFerrari.html", 5)
}

func TestLiveScrapeCategory(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://sexmex.xxx/tour/categories/milf.html", 5)
}
