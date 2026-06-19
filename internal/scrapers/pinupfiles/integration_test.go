//go:build integration

package pinupfiles

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLivePinupFiles(t *testing.T) {
	const u = "https://www.pinupfiles.com/"
	testutil.SkipIfPlaceholder(t, u)
	testutil.RunLiveScrape(t, New(), u, 2)
}
