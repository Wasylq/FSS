//go:build integration

package mplstudios

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveMPLStudios(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.mplstudios.com/videos/", 3)
}

func TestLiveMPLStudiosPortfolio(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.mplstudios.com/portfolio/290-Karissa_Diamond/", 3)
}
