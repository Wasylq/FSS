//go:build integration

package girlsgonegyno

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveGirlsGoneGyno(t *testing.T) {
	testutil.RunLiveScrape(t, New(sites[0]), "https://www.girlsgonegyno.com/", 3)
}

func TestLiveCaptiveClinic(t *testing.T) {
	testutil.RunLiveScrape(t, New(sites[1]), "https://www.captiveclinic.com/", 3)
}
