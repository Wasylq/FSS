//go:build integration

package allover30

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

const liveModelURL = "https://new.allover30.com/model-pages/ryan-keely/1549"

func TestLiveAllOver30(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveModelURL)
	testutil.RunLiveScrape(t, New(), liveModelURL, 2)
}
