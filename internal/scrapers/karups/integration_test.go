//go:build integration

package karups

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLive_KarupsOW(t *testing.T) {
	testutil.RunLiveScrape(t, newScraper(sites[0]), "https://www.karupsow.com/videos/", 5)
}

func TestLive_KarupsPC(t *testing.T) {
	testutil.RunLiveScrape(t, newScraper(sites[1]), "https://www.karupspc.com/videos/", 5)
}

func TestLive_KarupsHA(t *testing.T) {
	testutil.RunLiveScrape(t, newScraper(sites[2]), "https://www.karupsha.com/videos/", 5)
}

// Model pages route through runModel rather than the paginated walk. Both the
// canonical singular path and the plural alias (which the site 302s to the
// singular form) must be recognised — if the routing regex misses one, the
// scrape silently falls through and walks the entire site instead.
func TestLive_KarupsOWModelPageSingular(t *testing.T) {
	testutil.RunLiveScrape(t, newScraper(sites[0]),
		"https://www.karupsow.com/model/vitoria-vontese-7267.html", 2)
}

func TestLive_KarupsOWModelPagePlural(t *testing.T) {
	testutil.RunLiveScrape(t, newScraper(sites[0]),
		"https://www.karupsow.com/models/vitoria-vontese-7267.html", 2)
}
