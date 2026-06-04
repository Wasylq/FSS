//go:build integration

package faleno

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

const liveStudioURL = "https://falenogroup.com/makers/clover/"

func TestLiveFaleno(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}

func TestLiveDahlia(t *testing.T) {
	testutil.SkipIfPlaceholder(t, "https://dahlia-av.jp/")
	testutil.RunLiveScrape(t, New(), "https://dahlia-av.jp/", 2)
}
