//go:build integration

package ifeelmyself

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

// 14 > pageSize 12, so the main listing crosses a page boundary. The search and
// artist modes below stay small: they exercise different URL modes, not paging.
func TestLiveScrape(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://ifeelmyself.com", 14)
}

func TestLiveScrapeSearch(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://ifeelmyself.com/public/main.php?page=quick_search&keyword=Lucille_C", 3)
}

func TestLiveScrapeArtist(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://ifeelmyself.com/public/main.php?page=artist_bio&artist_id=f16900", 3)
}
