//go:build integration

package trans500

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveTrans500(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://trans500.com/", 2)
}

// A category id scrapes one StashDB child of the network — 44 is I Kill It TS.
func TestLiveTrans500Category(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://trans500.com/tour3/category.php?id=44", 2)
}

// A model page is a separate mode: one unpaginated listing.
func TestLiveTrans500Model(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://trans500.com/tour3/models/pamela-levinski.html", 1)
}
