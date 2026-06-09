//go:build integration

package watch4beauty

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveWatch4Beauty(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.watch4beauty.com/", 2)
}

func TestLiveWatch4BeautyModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.watch4beauty.com/model/erika-heiss", 2)
}
