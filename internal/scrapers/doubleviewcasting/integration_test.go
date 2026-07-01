//go:build integration

package doubleviewcasting

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveDoubleViewCasting(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "http://doubleviewcasting.com/scenes", 3)
}
