//go:build integration

package avidolz

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveAvIdolZ(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://avidolz.com", 3)
}
