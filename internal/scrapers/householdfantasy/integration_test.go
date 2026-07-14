//go:build integration

package householdfantasy

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveHouseholdFantasy(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://householdfantasy.com", 3)
}
