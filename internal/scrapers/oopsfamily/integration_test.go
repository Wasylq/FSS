//go:build integration

package oopsfamily

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrape(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://oopsfamily.com/", 5)
}

func TestLiveScrapeModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://oopsfamily.com/model/sophie-locke", 3)
}

func TestLiveScrapeTag(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://oopsfamily.com/tag/redhead", 3)
}
