//go:build integration

package zishy

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveZishy(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.zishy.com", 3)
}
