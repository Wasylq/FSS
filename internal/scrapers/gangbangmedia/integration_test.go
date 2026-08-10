//go:build integration

package gangbangmedia

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLive(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://p-p-p.tv/videos/list", 3)
}
