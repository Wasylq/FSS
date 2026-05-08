//go:build integration

package sexlikereal

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrape(t *testing.T) {
	s := New()
	testutil.RunLiveScrape(t, s, "https://www.sexlikereal.com/scenes", 2)
}

func TestLiveScrapeStudio(t *testing.T) {
	s := New()
	testutil.RunLiveScrape(t, s, "https://www.sexlikereal.com/studios/slr-originals-224", 2)
}

func TestLiveScrapeModel(t *testing.T) {
	s := New()
	testutil.RunLiveScrape(t, s, "https://www.sexlikereal.com/pornstars/molly-little-7099", 2)
}
