//go:build integration

package himeros

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveHimeros(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://himeros.tv/tour/", 2)
}
