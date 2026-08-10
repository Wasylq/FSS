//go:build integration

package pissplay

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLivePissPlay(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://pissplay.com/", 3)
}
