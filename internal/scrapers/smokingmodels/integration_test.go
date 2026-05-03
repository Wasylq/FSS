//go:build integration

package smokingmodels

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/railwayutil"
)

func TestLiveSmokingModels(t *testing.T) {
	railwayutil.RunLiveTest(t, New(), "https://smokingmodels.com/#/models", 5)
}
