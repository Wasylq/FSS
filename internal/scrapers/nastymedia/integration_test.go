//go:build integration

package nastymedia

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// All 4 sites share one HOME.html WWB layout, so two live smokes (the
// largest catalogue + a smaller sibling) validate the wiring without
// hammering the network.
func TestLiveCoozHound(t *testing.T) {
	cfg := findSite(t, "coozhound")
	testutil.RunLiveScrape(t, New(cfg), cfg.BaseURL+"/HOME.html", 4)
}

func TestLiveUrbanAmateurs(t *testing.T) {
	cfg := findSite(t, "urbanamateurs")
	testutil.RunLiveScrape(t, New(cfg), cfg.BaseURL+"/HOME.html", 4)
}

func findSite(t *testing.T, id string) SiteConfig {
	t.Helper()
	for _, cfg := range sites {
		if cfg.ID == id {
			return cfg
		}
	}
	t.Fatalf("no site config with ID %q", id)
	return SiteConfig{}
}
