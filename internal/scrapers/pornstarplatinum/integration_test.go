//go:build integration

package pornstarplatinum

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// One live smoke against the parent catalogue is enough — the whole
// network runs through the same `scenes.php` endpoint, so any sister-
// site URL would just exercise the same code path.
func TestLivePornstarPlatinum(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://pornstarplatinum.com/tour/scenes.php", 4)
}
