//go:build integration

package femjoy

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveFemjoy(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.femjoy.com/videos", 3)
}
