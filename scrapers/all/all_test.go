package all

import (
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

// This package is the public re-export that external modules blank-import to
// get the full catalogue (see docs/library.md). Its entire contract is the
// side effect of that import, so nothing here would fail to compile if the
// blank import were dropped — the registry would just come back empty for
// every downstream user. These tests are the only thing standing between that
// and a silent break.
func TestRegistryIsPopulated(t *testing.T) {
	got := scraper.All()
	if len(got) == 0 {
		t.Fatal("scraper.All() is empty — the blank import in this package is not registering anything")
	}
	// Sanity floor rather than an exact count: the real count lives in
	// internal/scrapers/all's TestReadmeScraperCount, and duplicating it here
	// would mean two files to update per new scraper.
	if len(got) < 100 {
		t.Errorf("scraper.All() returned %d scrapers, want the full catalogue", len(got))
	}
}

// Every registered scraper must be reachable by its own ID through the public
// package. Derived from the registry rather than hardcoding a site, so this
// cannot rot when a URL pattern changes.
func TestLookupThroughPublicPackage(t *testing.T) {
	for _, s := range scraper.All() {
		got, err := scraper.ForID(s.ID())
		if err != nil {
			t.Errorf("ForID(%q): %v", s.ID(), err)
			continue
		}
		if got.ID() != s.ID() {
			t.Errorf("ForID(%q) returned scraper %q", s.ID(), got.ID())
		}
	}
}
