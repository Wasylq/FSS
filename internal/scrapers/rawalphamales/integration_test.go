//go:build integration

package rawalphamales

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveRawAlphaMales(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.rawalphamales.com", 2)
}
