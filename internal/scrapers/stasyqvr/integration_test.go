//go:build integration

package stasyqvr

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveStasyQVR(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://stasyqvr.com/virtualreality/list?page=1", 3)
}
