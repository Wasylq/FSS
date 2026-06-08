//go:build integration

package waap

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

const liveURL = "https://www.waap.co.jp/work/search.php?serch=5&onrls=new&limit=45&pg=1"

func TestLiveWaap(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveURL)
	testutil.RunLiveScrape(t, New(), liveURL, 2)
}
