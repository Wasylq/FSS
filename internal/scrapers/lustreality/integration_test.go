//go:build integration

package lustreality

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveLustReality(t *testing.T) {
	t.Skip("lustreality.com is behind an Anubis proof-of-work interstitial — every HTML page returns the challenge, so the detail fetch cannot work without solving it")

	testutil.RunLiveScrape(t, New(), "https://www.lustreality.com/en/videos", 3)
}
