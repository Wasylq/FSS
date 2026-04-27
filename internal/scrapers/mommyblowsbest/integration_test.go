//go:build integration

package mommyblowsbest

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrapeAll(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.mommyblowsbest.com/", 5)
}
