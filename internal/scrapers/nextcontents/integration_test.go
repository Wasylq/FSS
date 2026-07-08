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
