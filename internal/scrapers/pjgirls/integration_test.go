//go:build integration

package pjgirls

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrape(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.pjgirls.com/sitemap.xml", 3)
}
