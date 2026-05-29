//go:build integration

package dirtyflix

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// One live smoke per parser variant.
//   - VariantThumbsItem: the parent dirtyflix.com (single-page catalogue).
//   - VariantBrutalX: brutalx.com.
//   - VariantThumbWrap: kinkyfamily.com (caption-text title flavour),
//     x-sensual.com (caption-h3 title flavour), privatecasting-x.com
//     (caption-text flavour, alternate domain form).

func TestLiveDirtyFlix(t *testing.T) {
	cfg := findSite(t, "dirtyflix")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

func TestLiveBrutalX(t *testing.T) {
	cfg := findSite(t, "brutalx")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

func TestLiveKinkyFamily(t *testing.T) {
	cfg := findSite(t, "kinkyfamily")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

func TestLiveXSensual(t *testing.T) {
	cfg := findSite(t, "xsensual")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

func TestLivePrivateCastingX(t *testing.T) {
	cfg := findSite(t, "privatecastingx")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

// VariantFuckingGlasses
func TestLiveFuckingGlasses(t *testing.T) {
	cfg := findSite(t, "fuckingglasses")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

// VariantBrutalX reused — massage-x + spypov use the same `<div id="N" class="th">`
// pattern with `<span class="title_thumb">` instead of `<h3>`.
func TestLiveMassageX(t *testing.T) {
	cfg := findSite(t, "massagex")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

func TestLiveSpyPOV(t *testing.T) {
	cfg := findSite(t, "spypov")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

// VariantMovieBlock — 5 sites use the same `<div class="movie-block">` cluster.
func TestLiveMakeHimCuckold(t *testing.T) {
	cfg := findSite(t, "makehimcuckold")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

func TestLiveSheIsNerdy(t *testing.T) {
	cfg := findSite(t, "sheisnerdy")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

func TestLiveMomsPassions(t *testing.T) {
	cfg := findSite(t, "momspassions")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

func TestLiveTrickYourGF(t *testing.T) {
	cfg := findSite(t, "trickyourgf")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

func TestLiveTrickyAgent(t *testing.T) {
	cfg := findSite(t, "trickyagent")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

// VariantYoungCourtesans
func TestLiveYoungCourtesans(t *testing.T) {
	cfg := findSite(t, "youngcourtesans")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

// VariantDebtsex
func TestLiveDebtsex(t *testing.T) {
	cfg := findSite(t, "debtsex")
	testutil.RunLiveScrape(t, New(cfg), cfg.SiteBase+"/", 4)
}

// VariantDisgrace
func TestLiveDisgraceThatBitch(t *testing.T) {
	cfg := findSite(t, "disgracethatbitch")
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
