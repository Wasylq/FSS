//go:build integration

package sexbabesvr

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestIntegration(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://sexbabesvr.com/", 5)
}
