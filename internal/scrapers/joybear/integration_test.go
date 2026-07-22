//go:build integration

package joybear

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveJoyBear(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.joybear.com", 3)
}
