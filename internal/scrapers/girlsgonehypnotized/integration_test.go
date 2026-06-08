//go:build integration

package girlsgonehypnotized

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveGirlsGoneHypnotized(t *testing.T) {
	const u = "https://girlsgonehypnotized.com/"
	testutil.SkipIfPlaceholder(t, u)
	testutil.RunLiveScrape(t, New(), u, 2)
}
