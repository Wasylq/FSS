//go:build integration

package babes

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// liveStudioURL — pick a real performer with a stable catalog.
// Pattern: https://www.babes.com/pornstar/<id>/<slug>
const liveStudioURL = "https://www.babes.com/model/3911/alexis-fawx"

func TestLiveBabes(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
