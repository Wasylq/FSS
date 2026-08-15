//go:build integration

package fratx

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveFratX(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://fratx.com/", 2)
}

// A category is one slice of the sweep, and a model set is a different listing
// shape; both are separate modes.
func TestLiveFratXCategory(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://fratx.com/category.php?id=9", 2)
}

func TestLiveFratXScene(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://fratx.com/trailer.php?id=455", 1)
}
