//go:build integration

package boyfun

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveBoyFun(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.boyfun.com/videos/", 3)
}
