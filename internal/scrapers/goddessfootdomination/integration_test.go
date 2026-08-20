//go:build integration

package goddessfootdomination

import (
	"strings"
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func find(t *testing.T, id string) SiteConfig {
	t.Helper()
	for _, cfg := range sites {
		if cfg.ID == id {
			return cfg
		}
	}
	t.Fatalf("no site config with ID %q", id)
	return SiteConfig{}
}

func TestLiveGoddessFootDomination(t *testing.T) {
	cfg := find(t, "goddessfootdomination")
	testutil.RunLiveScrape(t, New(cfg), cfg.BaseURL+"/", 4)
}

// The apex is not cosmetic: the certificate has a single SAN for it, so a www
// request fails verification before it is sent.
func TestLiveBaseURLIsTheApexHost(t *testing.T) {
	cfg := find(t, "goddessfootdomination")
	if strings.Contains(cfg.BaseURL, "//www.") {
		t.Errorf("BaseURL = %q, want the apex host", cfg.BaseURL)
	}
}

func TestLiveCategoryView(t *testing.T) {
	cfg := find(t, "goddessfootdomination")
	testutil.RunLiveScrape(t, New(cfg), cfg.BaseURL+"/category/footjobs", 2)
}
