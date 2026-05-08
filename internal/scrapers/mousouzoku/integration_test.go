//go:build integration

package mousouzoku

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrape(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.mousouzoku-av.com", 3)
}

func TestLiveScrapeMaker(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.mousouzoku-av.com/works/list/maker/462/", 3)
}
