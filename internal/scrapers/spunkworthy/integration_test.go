//go:build integration

package spunkworthy

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveSpunkWorthy(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.spunkworthy.com/preview/videos?page=1", 3)
}
