//go:build integration

package joannajet

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

// A single scene URL skips the sweep, which is the cheap live check.
func TestLiveJoannaJetScene(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "http://www.joannajet.com/scene_m.php?vid=1000", 1)
}

// The full sweep probes every vid up to the highest the index pages reach.
func TestLiveJoannaJet(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "http://www.joannajet.com/", 2)
}
