//go:build integration

package r18dev

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveR18devActress(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://r18.dev/videos/vod/movies/list/?id=1092582&type=actress", 2)
}

func TestLiveR18devStudio(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://r18.dev/videos/vod/movies/list/?id=40018&type=studio", 2)
}
