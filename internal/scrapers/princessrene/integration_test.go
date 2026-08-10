//go:build integration

package princessrene

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLivePrincessRene(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://worshiprene.com/videos/", 3)
}
