//go:build integration

package lustreality

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveLustReality(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.lustreality.com/en/videos", 3)
}
