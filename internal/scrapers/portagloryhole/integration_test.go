//go:build integration

package portagloryhole

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLivePortaGloryhole(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.portagloryhole.com/", 2)
}
