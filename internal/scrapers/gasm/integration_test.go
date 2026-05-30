//go:build integration

package gasm

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrape(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.gasm.com/studio/profile/cosplaybabes", 3)
}
