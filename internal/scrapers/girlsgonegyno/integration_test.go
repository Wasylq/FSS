//go:build integration

package girlsgonegyno

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLive(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.girlsgonegyno.com/", 3)
}
