//go:build integration

package takaratv

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveTakaraTVAll(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.takara-tv.jp/top_index.php", 2)
}

func TestLiveTakaraTVActress(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.takara-tv.jp/search.php?ac=712&search_flag=top", 2)
}
