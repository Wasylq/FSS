//go:build integration

package cosplayground

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveCosplayground(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://cosplayground.com/", 4)
}
