//go:build integration

package deeps

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveDeepsAll(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://deeps.net/item/", 2)
}

func TestLiveDeepsActress(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://deeps.net/item/index.php?w_葉山さゆり", 2)
}
