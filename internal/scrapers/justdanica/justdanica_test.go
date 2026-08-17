package justdanica

import "testing"

func TestIdentity(t *testing.T) {
	s := New()
	if s.ID() != "justdanica" {
		t.Errorf("ID() = %q", s.ID())
	}
	for _, u := range []string{
		"https://justdanica.com/",
		"https://www.justdanica.com/tour/",
		"https://www.justdanica.com/tour/updates/some-scene.html",
	} {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	for _, u := range []string{"https://danica.com/", "https://example.com/justdanica.com"} {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}
