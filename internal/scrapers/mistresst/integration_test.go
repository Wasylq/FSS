//go:build integration

package mistresst

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveMistressT(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.mistresst.net/", 3)
}
