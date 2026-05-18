//go:build integration

package porncz

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLivePornCZ(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.porncz.com/", 3)
}
