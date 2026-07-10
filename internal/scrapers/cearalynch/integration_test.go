//go:build integration

package cearalynch

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveCearaLynch(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.cearalynch.com/", 3)
}
