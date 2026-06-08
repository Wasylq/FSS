//go:build integration

package auntjudys

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

const liveStudioURL = "https://www.auntjudysxxx.com/tour/categories/movies.html"
const liveModelURL = "https://www.auntjudysxxx.com/tour/models/andi-james.html"
const liveNonXXXURL = "http://www.auntjudys.com/tour/categories/movies.html"

func TestLiveAuntJudys(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}

func TestLiveAuntJudysModel(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveModelURL)
	testutil.RunLiveScrape(t, New(), liveModelURL, 2)
}

func TestLiveAuntJudysNonXXX(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveNonXXXURL)
	testutil.RunLiveScrape(t, New(), liveNonXXXURL, 2)
}
