//go:build integration

package chickpass

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveChickPass(t *testing.T) {
	for _, cfg := range sites {
		if cfg.ID == "chickpass" {
			testutil.RunLiveScrape(t, New(cfg), "https://www.chickpass.com", 3)
			return
		}
	}
	t.Skip("chickpass config not found")
}
