//go:build integration

package dreamtranny

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveDreamTranny(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://dreamtranny.com/", 3)
}
