//go:build integration

package bamvisions

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveBAMVisions(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://tour.bamvisions.com/", 2)
}
