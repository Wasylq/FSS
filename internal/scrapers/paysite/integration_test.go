//go:build integration

package paysite

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveLadySonia(t *testing.T) {
	s := New(sites[0])
	testutil.RunLiveScrape(t, s, "https://tour.lady-sonia.com", 2)
}

func TestLiveMariskaX(t *testing.T) {
	s := New(sites[1])
	testutil.RunLiveScrape(t, s, "https://tour.mariskax.com", 2)
}

func TestLiveInkedPOV(t *testing.T) {
	s := New(sites[2])
	testutil.RunLiveScrape(t, s, "https://inkedpov.com", 2)
}

func TestLiveKaterinaHartlova(t *testing.T) {
	s := New(sites[3])
	testutil.RunLiveScrape(t, s, "https://tour.katerina-hartlova.com", 2)
}
