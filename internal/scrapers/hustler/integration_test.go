//go:build integration

package hustler

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveHustler(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://hustlerunlimited.com/", 3)
}
