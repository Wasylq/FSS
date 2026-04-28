//go:build integration

package faphouse

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveFapHouseModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://faphouse.com/models/angel-the-dreamgirl", 2)
}
