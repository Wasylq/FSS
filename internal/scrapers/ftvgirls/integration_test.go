//go:build integration

package ftvgirls

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveFTVGirls(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://ftvgirls.com/updates.html", 2)
}
