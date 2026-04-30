//go:build integration

package ideapocket

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveIdeaPocketSeries(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://ideapocket.com/works/list/series/833", 2)
}

func TestLiveIdeaPocketActress(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://ideapocket.com/actress/detail/868683", 2)
}
