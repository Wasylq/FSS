//go:build integration

package vintageflash

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveVintageFlash(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://vintageflash.com", 3)
}
