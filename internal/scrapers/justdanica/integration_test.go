//go:build integration

package justdanica

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveJustDanica(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.justdanica.com/tour/", 2)
}
