//go:build integration

package reaganfoxx

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

const liveStudioURL = "https://www.reaganfoxx.com/scenes/673608/reagan-foxx-streaming-pornstar-videos.html"

func TestLiveReaganFoxx(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
