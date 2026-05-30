//go:build integration

package czechav

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/czechavutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveCzechCasting(t *testing.T) {
	s := czechavutil.New(czechavutil.SiteConfig{SiteID: "czechcasting", Domain: "czechcasting.com", Studio: "Czech Casting"})
	testutil.RunLiveScrape(t, s, "https://czechcasting.com", 3)
}

func TestLiveHorrorPorn(t *testing.T) {
	s := czechavutil.New(czechavutil.SiteConfig{SiteID: "horrorporn", Domain: "horrorporn.com", Studio: "Horror Porn"})
	testutil.RunLiveScrape(t, s, "https://horrorporn.com", 3)
}

func TestLiveCzechStreets(t *testing.T) {
	s := czechavutil.New(czechavutil.SiteConfig{SiteID: "czechstreets", Domain: "czechstreets.com", Studio: "Czech Streets"})
	testutil.RunLiveScrape(t, s, "https://czechstreets.com", 3)
}
