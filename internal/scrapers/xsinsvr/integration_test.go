//go:build integration

package xsinsvr

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveXSinsVR(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://xsinsvr.com/videos", 3)
}

func TestLiveXSinsVRModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://xsinsvr.com/model/olivia-sparkle", 2)
}
