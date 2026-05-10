//go:build integration

package maturefetish

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrapeUpdates(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://maturefetish.com/en/updates", 5)
}

func TestLiveScrapeNiche(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://maturefetish.com/en/niche/67/1/facesitting", 3)
}

func TestLiveScrapeModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://maturefetish.com/en/model/10500/1/cresina", 2)
}
