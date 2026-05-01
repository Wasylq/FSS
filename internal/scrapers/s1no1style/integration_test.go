//go:build integration

package s1no1style

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveS1no1styleActress(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://s1s1s1.com/actress/detail/18906", 2)
}

func TestLiveS1no1styleRelease(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://s1s1s1.com/works/list/release", 2)
}
