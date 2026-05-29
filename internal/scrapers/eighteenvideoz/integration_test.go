//go:build integration

package eighteenvideoz

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// One live smoke per card-variant the network uses.
//
//   - 18videoz       — VariantA-parent + show_sets2 pagination.
//   - casualteensex  — VariantB-rich + show_sets2 pagination.
//   - teensanalyzed  — VariantB-rich + single-page (no pagination).
//   - teenylovers    — VariantA-child + show_sets pagination.
//   - younglibertines — VariantC-thumb (<div class="thumb">) + show_sets pagination.
//
// One live test per parser branch keeps coverage honest without spamming
// the upstream sites — sellyourgf and youngsexparties share the same
// VariantB-rich + show_sets2 path that casualteensex already exercises.

func TestLive18videoz(t *testing.T) {
	cfg := findSite(t, "18videoz")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

func TestLiveCasualTeenSex(t *testing.T) {
	cfg := findSite(t, "casualteensex")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

func TestLiveTeensAnalyzed(t *testing.T) {
	cfg := findSite(t, "teensanalyzed")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

func TestLiveTeenyLovers(t *testing.T) {
	cfg := findSite(t, "teenylovers")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

func TestLiveYoungLibertines(t *testing.T) {
	cfg := findSite(t, "younglibertines")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

// VariantD-table sites — legacy static-HTML layout, HTTP-only origin.
func TestLiveBangMyTeenAss(t *testing.T) {
	cfg := findSite(t, "bangmyteenass")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

func TestLiveFirstAnalDate(t *testing.T) {
	cfg := findSite(t, "firstanaldate")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

func TestLiveOldDicksYoungChix(t *testing.T) {
	cfg := findSite(t, "olddicksyoungchix")
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
