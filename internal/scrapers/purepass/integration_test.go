//go:build integration

package purepass

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLivePureCFNM(t *testing.T) {
	testutil.RunLiveScrape(t, New(sites[0]), "https://www.purecfnm.com/", 2)
}

func TestLiveAmateurCFNM(t *testing.T) {
	testutil.RunLiveScrape(t, New(sites[1]), "https://www.amateurcfnm.com/", 2)
}

func TestLiveCFNMGames(t *testing.T) {
	testutil.RunLiveScrape(t, New(sites[2]), "https://www.cfnmgames.com/", 2)
}

func TestLiveGirlsAbuseGuys(t *testing.T) {
	testutil.RunLiveScrape(t, New(sites[3]), "https://www.girlsabuseguys.com/", 2)
}

func TestLiveLadyVoyeurs(t *testing.T) {
	testutil.RunLiveScrape(t, New(sites[4]), "https://www.ladyvoyeurs.com/", 2)
}

func TestLiveLittleDickClub(t *testing.T) {
	testutil.RunLiveScrape(t, New(sites[5]), "https://littledick.club/", 2)
}
