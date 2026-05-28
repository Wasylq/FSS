//go:build integration

package privatecastings

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLivePrivateCastings(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.privatecastings.com/scenes", 4)
}
