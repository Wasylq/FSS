//go:build integration

package kmproduce

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveKMProduceVR(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.km-produce.com/works-vr/", 2)
}

func TestLiveKMProduceSell(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.km-produce.com/works-sell/", 2)
}

func TestLiveKMProduceTag(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.km-produce.com/works/tag/%e7%be%8e%e4%b9%b3", 2)
}

func TestLiveKMProduceActress(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.km-produce.com/nanase_alice", 2)
}
