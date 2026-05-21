//go:build integration

package glamose

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/internal/scrapers/utgutil"
)

func TestLiveHayleysSecrets(t *testing.T) {
	testutil.RunLiveScrape(t, utgutil.New(sites[0]), "https://hayleyssecrets.com/updates/videos", 2)
}

func TestLiveHayleysSecretsModel(t *testing.T) {
	testutil.RunLiveScrape(t, utgutil.New(sites[0]), "https://hayleyssecrets.com/models/hayley-marie-coppin", 2)
}

func TestLiveAllBrookWright(t *testing.T) {
	testutil.RunLiveScrape(t, utgutil.New(sites[5]), "https://www.allbrookwright.com/updates/videos", 2)
}

func TestLiveBethMorgan(t *testing.T) {
	testutil.RunLiveScrape(t, utgutil.New(sites[6]), "https://www.bethmorganofficial.com/updates/videos", 2)
}

func TestLiveSophia(t *testing.T) {
	testutil.RunLiveScrape(t, utgutil.New(sites[7]), "https://www.sophiassexylegwear.com/updates/videos", 2)
}

func TestLiveBreathTakers(t *testing.T) {
	testutil.RunLiveScrape(t, utgutil.New(sites[8]), "https://www.breath-takers.com/updates/videos", 2)
}

func TestLiveGirlfolio(t *testing.T) {
	testutil.RunLiveScrape(t, utgutil.New(sites[9]), "https://www.girlfolio.com/updates/videos", 2)
}
