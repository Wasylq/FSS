//go:build integration

package hmp

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveHMP(t *testing.T) {
	const u = "https://www.hmp.jp/"
	testutil.SkipIfPlaceholder(t, u)
	testutil.RunLiveScrape(t, New(), u, 2)
}
