package spizoo

import "testing"

func TestSiteCount(t *testing.T) {
	if len(sites) != 19 {
		t.Errorf("expected 19 sites, got %d", len(sites))
	}
}
