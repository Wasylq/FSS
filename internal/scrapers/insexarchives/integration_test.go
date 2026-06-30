//go:build integration

package insexarchives

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveInsexArchives(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "http://www.insexarchives.com/updates_new.php?start=0", 3)
}
