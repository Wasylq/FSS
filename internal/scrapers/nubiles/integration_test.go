//go:build integration

package nubiles

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// liveStudioURL — pick a real model with a stable catalog.
// Pattern: https://nubiles-porn.com/model/profile/<id>/<slug>
const liveStudioURL = "https://nubiles-porn.com/model/profile/2500/india-summer"

func TestLiveNubiles(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
