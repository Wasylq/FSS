//go:build integration

package dungeoncorp

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/dungeoncorputil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveSocietySM(t *testing.T) {
	testutil.RunLiveScrape(t, dungeoncorputil.New(sites[0]), "https://societysm.com/", 3)
}

func TestLiveCumbots(t *testing.T) {
	testutil.RunLiveScrape(t, dungeoncorputil.New(sites[1]), "https://cumbots.com/", 3)
}

func TestLiveFuckingDungeon(t *testing.T) {
	testutil.RunLiveScrape(t, dungeoncorputil.New(sites[2]), "https://www.dungeoncorp.com/?page=sites&site=FUD", 3)
}
