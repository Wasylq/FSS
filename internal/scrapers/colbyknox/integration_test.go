//go:build integration

package colbyknox

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveColbyKnox(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.colbyknox.com/videos", 3)
}
