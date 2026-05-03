//go:build integration

package spankingglamour

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/railwayutil"
)

func TestLiveSpankingGlamour(t *testing.T) {
	railwayutil.RunLiveTest(t, New(), "https://spankingglamour.com/#/models", 5)
}
