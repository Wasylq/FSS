//go:build integration

package littlecapricedreams

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveLittleCapriceDreams(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.littlecaprice-dreams.com/videos/", 2)
}

// Each of these takes a different branch: a collection filters the REST query
// by term, the bare sub-brand path is the alias the site redirects to that
// collection, and a model URL filters the walk by a slug set read off the
// model page.
func TestLiveCollection(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.littlecaprice-dreams.com/collection/buttmuse/", 2)
}

func TestLiveCollectionBareAlias(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.littlecaprice-dreams.com/streetfuck/", 2)
}

func TestLiveModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.littlecaprice-dreams.com/model/marcello-bravo/", 2)
}
