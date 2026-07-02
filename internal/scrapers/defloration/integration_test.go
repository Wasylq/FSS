//go:build integration

package defloration

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveDefloration(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.defloration.com/freetour.php?language=en", 3)
}
