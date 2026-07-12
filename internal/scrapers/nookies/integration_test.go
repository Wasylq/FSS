//go:build integration

package nookies

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

const liveStudioURL = "https://nookies.com/site/clubtug"

func TestLiveNookies(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveStudioURL)
	testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}

// TestLiveNookiesNewCMS exercises the VideoObject path on a brand that runs
// the new Laravel CMS on its own domain.
func TestLiveNookiesNewCMS(t *testing.T) {
	const u = "https://www.milfaf.com/"
	testutil.SkipIfPlaceholder(t, u)
	testutil.RunLiveScrape(t, New(), u, 3)
}

// TestLiveNookiesOver40Handjobs exercises the new-CMS path on over40handjobs.com,
// which migrated to this same Laravel/Vite platform and lacks a reliable
// VideoObject "genre" array (tag-pill fallback).
func TestLiveNookiesOver40Handjobs(t *testing.T) {
	const u = "https://www.over40handjobs.com/videos"
	testutil.SkipIfPlaceholder(t, u)
	testutil.RunLiveScrape(t, New(), u, 3)
}
