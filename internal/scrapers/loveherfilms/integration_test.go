//go:build integration

package loveherfilms

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveLoveHerFeet(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("loveherfeet"), "https://www.loveherfeet.com/tour/", 3)
}

func TestLiveLoveHerBoobs(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("loveherboobs"), "https://www.loveherboobs.com/tour/", 3)
}

func TestLiveLoveHerButt(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("loveherbutt"), "https://www.loveherbutt.com/tour/", 3)
}

func TestLiveSheLovesBlack(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("shelovesblack"), "https://www.shelovesblack.com/tour/", 3)
}

func TestLiveLoveHerFilms(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("loveherfilms"), "https://www.loveherfilms.com/tour/", 3)
}
