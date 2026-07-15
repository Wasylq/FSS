//go:build integration

package indiebucks

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// live looks a site up by ID rather than slice index — inserting a config row
// must not silently repoint these tests at a different site.
func live(t *testing.T, id string) {
	t.Helper()
	for _, c := range sites {
		if c.id == id {
			testutil.RunLiveScrape(t, newScraper(c), "https://www."+c.domain+"/videos", 3)
			return
		}
	}
	t.Fatalf("site not found: %s", id)
}

func TestLiveBoysSmoking(t *testing.T)      { live(t, "boyssmoking") }
func TestLiveBoysPissing(t *testing.T)      { live(t, "boyspissing") }
func TestLiveBoundMuscleJocks(t *testing.T) { live(t, "boundmusclejocks") }

// BoyNapped group — same Hollywood /videos template as the sites above.
func TestLiveBadBoyBondage(t *testing.T)     { live(t, "badboybondage") }
func TestLiveBadBoysBootcamp(t *testing.T)   { live(t, "badboysbootcamp") }
func TestLiveDaddysBondageBoys(t *testing.T) { live(t, "daddysbondageboys") }
func TestLiveUndieTwinks(t *testing.T)       { live(t, "undietwinks") }
