//go:build integration

package erosarts

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveJOI(t *testing.T) {
	testutil.RunLiveScrape(t, New(sites[0]), "https://jerkoffinstructions.com/", 2)
}

func TestLiveSexPOV(t *testing.T) {
	testutil.RunLiveScrape(t, New(sites[1]), "https://sexpov.com/", 2)
}

func TestLiveStepmomFun(t *testing.T) {
	testutil.RunLiveScrape(t, New(sites[2]), "https://stepmomfun.com/", 2)
}

func TestLiveTabooHandjobs(t *testing.T) {
	testutil.RunLiveScrape(t, New(sites[3]), "https://taboohandjobs.com/", 2)
}

func TestLiveTabooPOV(t *testing.T) {
	testutil.RunLiveScrape(t, New(sites[4]), "https://taboopov.com/", 2)
}
