//go:build integration

package lethalhardcore

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrapeAll(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.lethalhardcore.com/", 5)
}
