//go:build integration

package missax

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/missaxutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveMissaX(t *testing.T) {
	testutil.RunLiveScrape(t, missaxutil.New(sites[0]), "https://www.missax.com", 2)
}

func TestLiveAllHerLuv(t *testing.T) {
	testutil.RunLiveScrape(t, missaxutil.New(sites[1]), "https://www.allherluv.com/tour/", 2)
}
