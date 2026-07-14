//go:build integration

package d2pass

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

func TestLive1Pondo(t *testing.T)       { live(t, "1pondo") }
func TestLive10musume(t *testing.T)     { live(t, "10musume") }
func TestLivePacopacomama(t *testing.T) { live(t, "pacopacomama") }
func TestLiveMuramura(t *testing.T)     { live(t, "muramura") }
