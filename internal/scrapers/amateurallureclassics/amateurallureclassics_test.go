package amateurallureclassics

import "testing"

func TestID(t *testing.T) {
	if got := New().ID(); got != "amateurallureclassics" {
		t.Errorf("ID() = %q, want amateurallureclassics", got)
	}
}

func TestPatternsRootBase(t *testing.T) {
	if got := New().Patterns()[0]; got != "amateurallureclassics.com/" {
		t.Errorf("Patterns()[0] = %q, want amateurallureclassics.com/", got)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := map[string]bool{
		"https://www.amateurallureclassics.com/categories/movies_1_d.html": true,
		"https://amateurallureclassics.com/scenes/foo_vids.html":           true,
		"https://amateurallure.com/":                                       false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}
