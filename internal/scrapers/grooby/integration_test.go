//go:build integration

package grooby

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/groobyutil"
	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveGroobyGirls(t *testing.T) {
	testutil.RunLiveScrape(t, groobyutil.New(sites[13]), "https://www.groobygirls.com/tour/", 2)
}
