//go:build integration

package maturenl

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrapeUpdates(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.mature.nl/en/updates", 5)
}

func TestLiveScrapeNiche(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.mature.nl/en/niche/570/1/4k", 5)
}

func TestLiveScrapeModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.mature.nl/en/model/9071", 3)
}
