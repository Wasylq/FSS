//go:build integration

package jeffsmodels

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveJeffsModels(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://jeffsmodels.com/", 3)
}
