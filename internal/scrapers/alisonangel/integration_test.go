//go:build integration

package alisonangel

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveAlisonAngel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://alisonangel.com/", 3)
}
