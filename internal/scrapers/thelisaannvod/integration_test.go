//go:build integration

package thelisaannvod

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveTheLisaAnnVOD(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://thelisaann.com/vod/", 3)
}

// A model URL walks a different path stem through the same pager.
func TestLiveTheLisaAnnVODModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://thelisaann.com/vod/models/lisa-ann.html", 3)
}
