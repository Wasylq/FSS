//go:build integration

package frenchtwinks

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveFrenchTwinks(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.french-twinks.com/en/", 3)
}
