//go:build integration

package mofos

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// liveStudioURL — pick a real performer with a stable catalog.
// Pattern: https://www.mofos.com/pornstar/<id>/<slug>
const liveStudioURL = "https://www.mofos.com/model/535/cory-chase"

func TestLiveMofos(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
