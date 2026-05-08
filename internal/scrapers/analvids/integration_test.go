//go:build integration

package analvids

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrape(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.analvids.com/new-videos", 2)
}

func TestLiveScrapeStudio(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.analvids.com/studios/giorgio-grandi", 2)
}
