//go:build integration

package sensationalvideo

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/masutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrape(t *testing.T) {
	s := masutil.New(sites[0]) // plumperpass
	testutil.RunLiveScrape(t, s, "https://www.plumperpass.com", 5)
}

func TestLiveScrapeBBWsGoneBlack(t *testing.T) {
	s := masutil.New(sites[3]) // bbwsgoneblack
	testutil.RunLiveScrape(t, s, "https://www.bbwsgoneblack.com", 5)
}
