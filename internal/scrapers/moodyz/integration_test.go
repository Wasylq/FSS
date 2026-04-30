//go:build integration

package moodyz

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveMoodyzSeries(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://moodyz.com/works/list/series/3482", 2)
}

func TestLiveMoodyzActress(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://moodyz.com/actress/detail/700115", 2)
}
