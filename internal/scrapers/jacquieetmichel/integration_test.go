//go:build integration

package jacquieetmichel

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveJacquieEtMichel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.jacquieetmicheltv.net/", 2)
}
