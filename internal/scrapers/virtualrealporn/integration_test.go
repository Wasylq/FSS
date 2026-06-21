//go:build integration

package virtualrealporn

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveVirtualRealPorn(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("virtualrealporn"), "https://virtualrealporn.com/", 3)
}

func TestLiveVirtualRealGay(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("virtualrealgay"), "https://virtualrealgay.com/", 3)
}

func TestLiveVirtualRealTrans(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("virtualrealtrans"), "https://virtualrealtrans.com/", 3)
}

func TestLiveVirtualRealJapan(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("virtualrealjapan"), "https://virtualrealjapan.com/", 3)
}

func TestLiveVirtualRealPassion(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("virtualrealpassion"), "https://virtualrealpassion.com/", 3)
}
