//go:build integration

package ad4x

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveAD4X(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://ad4x.com/en/videos", 3)
}

func TestLiveAD4XModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://ad4x.com/en/models/lexi", 2)
}
