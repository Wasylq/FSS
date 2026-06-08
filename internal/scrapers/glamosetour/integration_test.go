//go:build integration

package glamosetour

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLivePantyAmateur(t *testing.T) {
	testutil.RunLiveScrape(t, New(sites[6]), "https://www.pantyamateur.com/", 2)
}
