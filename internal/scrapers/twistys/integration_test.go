//go:build integration

package twistys

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrapeAll(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.twistys.com/", 5)
}
