package swallowsalon

import "testing"

func TestID(t *testing.T) {
	if got := New().ID(); got != "swallowsalon" {
		t.Errorf("ID() = %q, want swallowsalon", got)
	}
}

func TestPatternsRootBase(t *testing.T) {
	// Swallow Salon serves the tour from the document root, so patterns must
	// not carry the default "/trial" prefix.
	for _, p := range New().Patterns() {
		if len(p) >= len("swallowsalon.com/trial") && p[:len("swallowsalon.com/trial")] == "swallowsalon.com/trial" {
			t.Errorf("pattern %q should not contain /trial prefix", p)
		}
	}
	if got := New().Patterns()[0]; got != "swallowsalon.com/" {
		t.Errorf("Patterns()[0] = %q, want swallowsalon.com/", got)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := map[string]bool{
		"https://www.swallowsalon.com/categories/movies_1_d.html": true,
		"https://swallowsalon.com/scenes/foo_vids.html":           true,
		"https://example.com/swallowsalon":                        false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}
