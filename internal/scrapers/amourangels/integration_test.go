//go:build integration

package amourangels

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveAmourAngels(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "http://amourangels.com/", 3)
}
