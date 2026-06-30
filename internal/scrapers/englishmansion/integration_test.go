//go:build integration

package englishmansion

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveEnglishMansion(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.theenglishmansion.com/updates.html", 3)
}
