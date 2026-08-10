//go:build integration

package realjamvr

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestIntegration(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://realjamvr.com/", 5)
}
