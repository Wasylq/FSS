//go:build integration

package privateclassics

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLivePrivateClassics(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.privateclassics.com/en/scenes/", 4)
}

func TestLivePrivateClassicsPornstar(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.privateclassics.com/en/pornstar/1-silvia-saint", 2)
}
