//go:build integration

package sketchysex

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveSketchySex(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://sketchysex.com/", 3)
}
