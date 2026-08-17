//go:build integration

package bananafever

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveBananaFever(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://bananafever.com/", 2)
}

// A single scene URL skips the sitemap.
func TestLiveBananaFeverScene(t *testing.T) {
	testutil.RunLiveScrape(t, New(),
		"https://bananafever.com/video/hot-young-blonde-21-year-old-step-sister-chloe-1-2-5ZG9KY", 1)
}
