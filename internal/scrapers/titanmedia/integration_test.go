//go:build integration

package titanmedia

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/internal/scrapers/titanmediautil"
)

func TestLiveGloryholeSwallow(t *testing.T) {
	testutil.RunLiveScrape(t, titanmediautil.New(sites[0]), "https://gloryholeswallow.com/", 3)
}

func TestLiveCumpsters(t *testing.T) {
	testutil.RunLiveScrape(t, titanmediautil.New(sites[2]), "https://cumpsters.com/", 3)
}
