//go:build integration

package puffy

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/puffyutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveWetAndPuffy(t *testing.T) {
	testutil.RunLiveScrape(t, puffyutil.New(sites[0]), "https://wetandpuffy.com/", 3)
}

func TestLiveVIPissy(t *testing.T) {
	testutil.RunLiveScrape(t, puffyutil.New(sites[4]), "https://vipissy.com/", 3)
}
