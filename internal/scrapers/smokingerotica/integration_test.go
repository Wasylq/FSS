//go:build integration

package smokingerotica

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/railwayutil"
)

func TestLiveSmokingErotica(t *testing.T) {
	railwayutil.RunLiveTest(t, New(), "https://smokingerotica.com/#/models", 5)
}
