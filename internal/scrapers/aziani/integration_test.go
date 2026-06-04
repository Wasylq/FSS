//go:build integration

package aziani

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveAziani(t *testing.T) {
	testutil.RunLiveScrape(t, New(sites[0]), "https://www.aziani.com/", 2)
}
