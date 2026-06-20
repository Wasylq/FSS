//go:build integration

package badoink

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveBadoinkVR(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("badoinkvr"), "https://badoinkvr.com/vrpornvideos", 3)
}

func TestLive18VR(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("18vr"), "https://18vr.com/vrpornvideos", 3)
}

func TestLiveVRCosplayX(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("vrcosplayx"), "https://vrcosplayx.com/cosplaypornvideos", 3)
}

func TestLiveBabeVR(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("babevr"), "https://babevr.com/vrpornvideos", 3)
}

func TestLiveRealVR(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("realvr"), "https://realvr.com/vrpornvideos", 3)
}
