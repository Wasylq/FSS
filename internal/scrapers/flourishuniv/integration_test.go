//go:build integration

package flourishuniv

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveFlourishUniv(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.flourishuniv.com/episodes/", 3)
}
