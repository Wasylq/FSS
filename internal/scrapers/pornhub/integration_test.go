//go:build integration

package pornhub

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// liveStudioURL — pick a real pornstar or channel.
// Patterns: https://www.pornhub.com/pornstar/<slug>
//
//	https://www.pornhub.com/channels/<slug>
const liveStudioURL = "https://www.pornhub.com/pornstar/coco-vandi"

func TestLivePornhub(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
