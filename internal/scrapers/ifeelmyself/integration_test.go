//go:build integration

package ifeelmyself

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrape(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://ifeelmyself.com", 5)
}

func TestLiveScrapeArtist(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://ifeelmyself.com/public/main.php?page=artist_bio&artist_id=HME2FC", 3)
}
