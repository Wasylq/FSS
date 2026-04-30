//go:build integration

package madonna

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveMadonnaSeries(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://madonna-av.com/works/list/series/1546", 2)
}

func TestLiveMadonnaActress(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://madonna-av.com/actress/detail/216942", 2)
}
