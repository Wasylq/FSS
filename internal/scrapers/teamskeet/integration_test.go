//go:build integration

package teamskeet

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveTeamSkeet(t *testing.T) {
	s := newSiteScraper(sites[0]) // teamskeet
	testutil.RunLiveScrape(t, s, "https://www.teamskeet.com/", 2)
}

func TestLiveTeamSkeetSeries(t *testing.T) {
	s := newSiteScraper(sites[0])
	testutil.RunLiveScrape(t, s, "https://www.teamskeet.com/series/exxxtrasmall", 2)
}

func TestLiveMYLF(t *testing.T) {
	s := newSiteScraper(sites[1]) // mylf
	testutil.RunLiveScrape(t, s, "https://www.mylf.com/", 2)
}

func TestLiveMYLFSeries(t *testing.T) {
	s := newSiteScraper(sites[1])
	testutil.RunLiveScrape(t, s, "https://www.mylf.com/series/milfty-ts", 2)
}

func TestLiveFamilyStrokes(t *testing.T) {
	s := newSiteScraper(sites[2]) // familystrokes
	testutil.RunLiveScrape(t, s, "https://www.familystrokes.com/", 2)
}

func TestLivePervz(t *testing.T) {
	s := newSiteScraper(sites[4]) // pervz
	testutil.RunLiveScrape(t, s, "https://www.pervz.com/", 2)
}
