//go:build integration

package caribbeancom

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveCaribbeancom(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.caribbeancom.com/listpages/all1.htm", 3)
}
