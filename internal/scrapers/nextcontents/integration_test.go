//go:build integration

package nextcontents

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func live(t *testing.T, id string) {
	for _, c := range sites {
		if c.SiteID == id {
			testutil.RunLiveScrape(t, newScraper(c), c.Base+"/"+c.ListPath, 3)
			return
		}
	}
	t.Fatalf("site not found: %s", id)
}

func TestLiveFreakMob(t *testing.T)         { live(t, "freakmob") }
func TestLiveDeepthroatSirens(t *testing.T) { live(t, "deepthroatsirens") }
func TestLiveSwallowed(t *testing.T)        { live(t, "swallowed") }

// Sticky Dollars network.
func TestLiveTrueAnal(t *testing.T)       { live(t, "trueanal") }
func TestLiveNympho(t *testing.T)         { live(t, "nympho") }
func TestLiveDirtyAuditions(t *testing.T) { live(t, "dirtyauditions") }
func TestLiveAllAnal(t *testing.T)        { live(t, "allanal") }
func TestLiveAnalOnly(t *testing.T)       { live(t, "analonly") }

// Top Web Models network.
func TestLiveBigGulpGirls(t *testing.T)   { live(t, "biggulpgirls") }
func TestLiveShesBrandNew(t *testing.T)   { live(t, "shesbrandnew") }
func TestLiveFacialsForever(t *testing.T) { live(t, "facialsforever") }
func TestLiveCougarSeason(t *testing.T)   { live(t, "cougarseason") }
func TestLivePoundedPetite(t *testing.T)  { live(t, "poundedpetite") }
func TestLive2Girls1Camera(t *testing.T)  { live(t, "2girls1camera") }
func TestLiveTopWebModels(t *testing.T)   { live(t, "topwebmodels") }
func TestLiveTWMClassics(t *testing.T)    { live(t, "twmclassics") }
func TestLiveTWMInterviews(t *testing.T)  { live(t, "twminterviews") }
func TestLiveTWMPornVault(t *testing.T)   { live(t, "twmpornvault") }

// AltErotic.
func TestLiveAltErotic(t *testing.T) { live(t, "alterotic") }

// Blake Mason (ex-Twisted XXX Media).
func TestLiveBlakeMason(t *testing.T) { live(t, "blakemason") }
