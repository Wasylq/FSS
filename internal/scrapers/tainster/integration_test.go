//go:build integration

package tainster

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveSinxAll(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.sinx.com/videos/all", 3)
}

func TestLiveSinxChannel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.sinx.com/Slime-Wave", 2)
}

func TestLiveSinxPerformer(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.sinx.com/girls/40-Victoria-Rose", 2)
}

func TestLiveSinxTag(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.sinx.com/tag/521-orgy", 2)
}
