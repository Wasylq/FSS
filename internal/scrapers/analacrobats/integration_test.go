//go:build integration

package analacrobats

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveAnalAcrobats(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.analacrobats.com/", 2)
}
