//go:build integration

package mercury

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveMercury(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://mercury.diary.to", 3)
}
