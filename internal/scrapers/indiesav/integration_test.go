//go:build integration

package indiesav

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveYoungPeach(t *testing.T) {
	const u = "https://www.indies-av.co.jp/lables/ymdd/"
	testutil.SkipIfPlaceholder(t, u)
	testutil.RunLiveScrape(t, New(), u, 2)
}

func TestLiveYoungPeach2(t *testing.T) {
	const u = "https://www.indies-av.co.jp/lables/ymds/"
	testutil.SkipIfPlaceholder(t, u)
	testutil.RunLiveScrape(t, New(), u, 2)
}
