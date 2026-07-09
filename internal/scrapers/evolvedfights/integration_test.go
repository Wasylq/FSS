//go:build integration

package evolvedfights

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func live(t *testing.T, id string) {
	for _, c := range sites {
		if c.SiteID == id {
			testutil.RunLiveScrape(t, newScraper(c), c.Base+"/categories/"+c.Listing+"_1_"+c.Sort+".html", 3)
			return
		}
	}
	t.Fatalf("site not found: %s", id)
}

func TestLiveEvolvedFights(t *testing.T) { live(t, "evolvedfights") }
