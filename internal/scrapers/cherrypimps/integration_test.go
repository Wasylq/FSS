//go:build integration

package cherrypimps

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/cherrypimpsutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveCherryPimps(t *testing.T) {
	testutil.RunLiveScrape(t, cherrypimpsutil.New(sites[0]), "https://cherrypimps.com/", 2)
}
