//go:build integration

package pornbox

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveStudio(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://pornbox.com/application/studio/123", 4)
}

func TestLiveModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://pornbox.com/application/model/5339", 4)
}
