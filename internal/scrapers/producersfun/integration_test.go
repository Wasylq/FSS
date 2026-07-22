//go:build integration

package producersfun

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveProducersFun(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://producersfun.com", 3)
}
