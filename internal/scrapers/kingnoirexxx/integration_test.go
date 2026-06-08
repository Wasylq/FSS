//go:build integration

package kingnoirexxx

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveKingNoireXXX(t *testing.T) {
	const u = "https://kingnoirexxx.com"
	testutil.SkipIfPlaceholder(t, u)
	testutil.RunLiveScrape(t, New(), u, 2)
}
