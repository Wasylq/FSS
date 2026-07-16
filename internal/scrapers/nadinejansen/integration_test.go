//go:build integration

package nadinejansen

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveNadineJansen(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://nadine-j.de/models/videos", 3)
}
