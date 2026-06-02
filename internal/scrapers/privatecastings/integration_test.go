//go:build integration

package privatecastings

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLivePrivateCastings(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.privatecastings.com/scenes", 4)
}

func TestLivePrivateCastingsPornstar(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.privatecastings.com/pornstar/1-gina-gerson/", 2)
}
