//go:build integration

package erikalust

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveErikaLust(t *testing.T) {
	testutil.RunLiveScrape(t, New(sites[0]), "https://erikalust.com/", 2)
}

func TestLiveLustCinema(t *testing.T) {
	testutil.RunLiveScrape(t, New(sites[1]), "https://lustcinema.com/", 2)
}

func TestLiveXConfessions(t *testing.T) {
	testutil.RunLiveScrape(t, New(sites[2]), "https://xconfessions.com/", 2)
}

// A single scene URL skips the sitemaps.
func TestLiveXConfessionsFilm(t *testing.T) {
	testutil.RunLiveScrape(t, New(sites[2]), "https://xconfessions.com/film/my-pregnant-desire", 1)
}
