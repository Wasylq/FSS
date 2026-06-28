//go:build integration

package perfectgonzo

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/perfectgonzoutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveAllInternal(t *testing.T) {
	testutil.RunLiveScrape(t, perfectgonzoutil.New(sites[0]), "http://www.allinternal.com/movies", 3)
}

func TestLiveAssTraffic(t *testing.T) {
	testutil.RunLiveScrape(t, perfectgonzoutil.New(sites[1]), "http://www.asstraffic.com/movies", 3)
}
