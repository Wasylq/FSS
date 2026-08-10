//go:build integration

package glosstightsglamour

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveGlossTightsGlamour(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.glosstightsglamour.com/", 3)
}
