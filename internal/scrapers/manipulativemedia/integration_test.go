//go:build integration

package manipulativemedia

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveMyPervyFamily(t *testing.T) {
	testutil.RunLiveScrape(t, NewMyPervyFamily(), "https://www.mypervyfamily.com", 3)
}

func TestLiveTouchMyWife(t *testing.T) {
	testutil.RunLiveScrape(t, NewTouchMyWife(), "https://www.touchmywife.com", 3)
}
