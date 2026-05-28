//go:build integration

package ghostproclassic

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// One smoke per representative site. All four sister sites share the same
// parser so per-site verification is covered by the offline table test
// TestSitesTable_uniqueIDsAndDomainIsolation.
func TestLiveCreampieThais(t *testing.T) {
	cfg := findSite(t, "creampiethais")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

func TestLiveMongerInAsia(t *testing.T) {
	cfg := findSite(t, "mongerinasia")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

func findSite(t *testing.T, id string) SiteConfig {
	t.Helper()
	for _, cfg := range sites {
		if cfg.ID == id {
			return cfg
		}
	}
	t.Fatalf("no site config registered with ID %q", id)
	return SiteConfig{}
}
