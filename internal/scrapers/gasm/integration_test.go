//go:build integration

package gasm

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// gasm has two URL modes and 13 patterns: the gasm.com studio-profile path, and
// sibling domains resolved through domainToSlug. Only the profile mode was
// covered, leaving the whole domain-mapping path untested — a wrong or missing
// domainToSlug entry would have gone unnoticed.

func TestLiveScrape(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.gasm.com/studio/profile/cosplaybabes", 3)
}

// A second profile, so the mode is not proven by a single studio.
func TestLiveScrapeSecondProfile(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.gasm.com/studio/profile/mmvfilms", 2)
}

// The sibling-domain mode: resolved via domainToSlug rather than the URL path.
func TestLiveSiblingDomain(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://mmvfilms.com/", 2)
}
