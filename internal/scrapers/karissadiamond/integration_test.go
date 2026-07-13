//go:build integration

package karissadiamond

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveKarissaDiamond(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://karissa-diamond.com/videoCollection/", 3)
}
