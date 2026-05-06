package cherrypimps

import "testing"

func TestSiteCount(t *testing.T) {
	if len(sites) != 2 {
		t.Errorf("got %d sites, want 2", len(sites))
	}
}

func TestUniqueSiteIDs(t *testing.T) {
	seen := make(map[string]bool)
	for _, s := range sites {
		if seen[s.ID] {
			t.Errorf("duplicate site ID: %s", s.ID)
		}
		seen[s.ID] = true
	}
}
