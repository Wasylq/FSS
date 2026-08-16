//go:build integration

package porndudecasting

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLivePornDudeCasting(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://porndudecasting.com/", 2)
}
