//go:build integration

package tadpolexstudio

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveTadPoleXStudio(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.tadpolexstudio.com/categories/movies.html", 3)
}
