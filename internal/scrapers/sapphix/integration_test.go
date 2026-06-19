//go:build integration

package sapphix

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveSapphicErotica(t *testing.T) {
	const u = "https://www.sapphicerotica.com/"
	testutil.SkipIfPlaceholder(t, u)
	testutil.RunLiveScrape(t, newFor("sapphicerotica"), u, 2)
}

func TestLiveSapphix(t *testing.T) {
	const u = "https://www.sapphix.com/"
	testutil.SkipIfPlaceholder(t, u)
	testutil.RunLiveScrape(t, newFor("sapphix"), u, 2)
}

func TestLiveFistFlush(t *testing.T) {
	const u = "https://www.fistflush.com/"
	testutil.SkipIfPlaceholder(t, u)
	testutil.RunLiveScrape(t, newFor("fistflush"), u, 2)
}

func TestLiveGiveMePink(t *testing.T) {
	const u = "https://www.givemepink.com/"
	testutil.SkipIfPlaceholder(t, u)
	testutil.RunLiveScrape(t, newFor("givemepink"), u, 2)
}
