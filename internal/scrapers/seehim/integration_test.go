//go:build integration

package seehim

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func live(t *testing.T, id string) {
	t.Helper()
	for _, c := range sites {
		if c.SiteID == id {
			testutil.RunLiveScrape(t, newScraper(c), "https://"+c.Domain+"/", 3)
			return
		}
	}
	t.Fatalf("site not found: %s", id)
}

func TestLiveSeeHimFuck(t *testing.T) { live(t, "seehimfuck") }
func TestLiveSeeHimSolo(t *testing.T) { live(t, "seehimsolo") }
func TestLiveRaveBunnys(t *testing.T) { live(t, "ravebunnys") }
