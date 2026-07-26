//go:build integration

package camsoda

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveCamSodaExclusiveVideos(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.camsoda.com/exclusive-videos", 2)
}

func TestLiveCamSodaModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.camsoda.com/nicollemeyer", 2)
}
