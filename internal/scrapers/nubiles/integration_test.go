//go:build integration

package nubiles

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// nubiles advertises 51 patterns: three URL modes on nubiles-porn.com plus ~48
// sibling domains, all handled by the same scraper. Only the model-profile mode
// was covered, so the main listing, the category mode and every other domain
// went unexercised — a break in any of them would have gone unnoticed.
//
// These cover the three *modes* plus two representative sibling domains. The
// remaining ~46 domains are deliberately not each given a test: they differ only
// by hostname and testing them all would multiply smoke runtime for no extra
// signal. Rotate the two below if a specific site is suspect.

// liveStudioURL — a real model with a stable catalogue.
// Pattern: https://nubiles-porn.com/model/profile/<id>/<slug>
const liveStudioURL = "https://nubiles-porn.com/model/profile/2500/india-summer"

func TestLiveNubiles(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}

// The main listing: the default mode, and the one a bare studio URL hits.
func TestLiveNubilesMainListing(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://nubiles-porn.com/", 2)
}

// The category mode, which resolves through categoryRe rather than modelRe.
func TestLiveNubilesCategory(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://nubiles-porn.com/video/category/2213/european-vacation", 2)
}

// A sibling domain: the scraper derives its base from the studio URL, so a
// second host proves that resolution rather than re-testing nubiles-porn.com.
func TestLiveNubilesSiblingDomain(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://nubiles.net/", 2)
}

func TestLiveMomsTeachSex(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://momsteachsex.com/", 2)
}
