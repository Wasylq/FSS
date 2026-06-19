//go:build integration

package bluedonkey

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveKimHolland(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("kimholland"), "https://www.kimholland.com/videos/", 2)
}

func TestLiveMeidenVanHolland(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("meidenvanholland"), "https://meidenvanholland.nl/sexfilms", 2)
}

func TestLiveVurigVlaanderen(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("vurigvlaanderen"), "https://vurigvlaanderen.be/sexfilms", 2)
}

func TestLiveSecretCircle(t *testing.T) {
	testutil.RunLiveScrape(t, newFor("secretcircle"), "https://secretcircle.com/seksfilms", 2)
}
