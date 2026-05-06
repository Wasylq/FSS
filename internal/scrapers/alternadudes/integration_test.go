//go:build integration

package alternadudes

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLive_AlternaDudes(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.alternadudes.com/categories/movies.html", 5)
}
