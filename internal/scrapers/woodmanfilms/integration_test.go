//go:build integration

package woodmanfilms

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveWoodmanFilms(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.woodmanfilms.com/", 2)
}
