//go:build integration

package bratprincess

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveBratPrincess(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.bratprincess.us/video-list", 3)
}
