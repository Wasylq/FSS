//go:build integration

package xxxfollow

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveXXXFollow(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.xxxfollow.com/katanakombat/premium", 2)
}
