//go:build integration

package firstanalquest

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveFirstAnalQuest(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "http://www.firstanalquest.com/latest-updates/", 3)
}
