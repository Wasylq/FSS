//go:build integration

package jizzonteens

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveJizzOnTeens(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://jizzonteens.com", 3)
}
