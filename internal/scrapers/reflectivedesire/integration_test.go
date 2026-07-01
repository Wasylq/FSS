//go:build integration

package reflectivedesire

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveReflectiveDesire(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://reflectivedesire.com/", 3)
}
