//go:build integration

package tokyohot

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveTokyoHot(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.tokyo-hot.com/product/", 3)
}
