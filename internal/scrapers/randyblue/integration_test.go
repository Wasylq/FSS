//go:build integration

package randyblue

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveRandyBlue(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.randyblue.com/categories/videos_1_d.html", 3)
}
