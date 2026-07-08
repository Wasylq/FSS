//go:build integration

package fuckermate

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveFuckermate(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.fuckermate.com/video", 3)
}
