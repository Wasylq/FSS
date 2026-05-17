//go:build integration

package yummygirl

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveYummyGirl(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://yummygirl.com/", 2)
}

func TestLiveYummyGirlModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://yummygirl.com/models/sofie-marie.html", 2)
}
