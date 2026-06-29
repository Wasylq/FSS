//go:build integration

package vrhush

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestIntegration(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://vrhush.com/", 5)
}
