//go:build integration

package scissorgoddess

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveScissorGoddess(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://scissorgoddess.net", 3)
}
