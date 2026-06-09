//go:build integration

package playboyplus

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLivePlayboyPlus(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.playboyplus.com/", 2)
}

func TestLivePlayboyPlusModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.playboyplus.com/en/model/view/Alana-Rey/123147", 2)
}
