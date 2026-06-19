//go:build integration

package blumedia

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveBrokeStraightBoys(t *testing.T) {
	const u = "https://www.brokestraightboys.com/"
	testutil.SkipIfPlaceholder(t, u)
	testutil.RunLiveScrape(t, newFor("brokestraightboys"), u, 2)
}

func TestLiveBoyGusher(t *testing.T) {
	const u = "https://www.boygusher.com/"
	testutil.SkipIfPlaceholder(t, u)
	testutil.RunLiveScrape(t, newFor("boygusher"), u, 2)
}

func TestLiveCollegeBoyPhysicals(t *testing.T) {
	const u = "https://www.collegeboyphysicals.com/"
	testutil.SkipIfPlaceholder(t, u)
	testutil.RunLiveScrape(t, newFor("collegeboyphysicals"), u, 2)
}

func TestLiveCollegeDudes(t *testing.T) {
	const u = "https://www.collegedudes.com/"
	testutil.SkipIfPlaceholder(t, u)
	testutil.RunLiveScrape(t, newFor("collegedudes"), u, 2)
}
