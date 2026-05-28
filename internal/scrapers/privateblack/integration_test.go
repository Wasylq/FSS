//go:build integration

package privateblack

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLivePrivateBlack(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.privateblack.com/scenes", 4)
}
