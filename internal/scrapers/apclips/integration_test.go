//go:build integration

package apclips

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveAPClips(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://apclips.com/ashleymason973", 2)
}
