//go:build integration

package peatv

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLivePEATV(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://pea-tv.jp/search.php?b=1", 2)
}
