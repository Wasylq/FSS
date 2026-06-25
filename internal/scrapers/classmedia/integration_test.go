//go:build integration

package classmedia

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveSubspaceland(t *testing.T) {
	testutil.RunLiveScrape(t, NewSubspaceland(), "https://www.subspaceland.com", 3)
}

func TestLiveOldje(t *testing.T) {
	testutil.RunLiveScrape(t, NewOldje(), "https://www.oldje.com", 3)
}

func TestLiveOldje3some(t *testing.T) {
	testutil.RunLiveScrape(t, NewOldje3some(), "https://www.oldje-3some.com", 3)
}
