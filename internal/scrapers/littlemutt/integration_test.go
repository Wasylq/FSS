//go:build integration

package littlemutt

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveLittleMutt(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "http://tour.littlemutt.com/videos/", 3)
}
