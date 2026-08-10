//go:build integration

package wearehairy

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveWeAreHairy(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://wearehairy.com/categories/Photos", 3)
}
