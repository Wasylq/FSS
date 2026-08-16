package himeros

import "testing"

func TestIdentity(t *testing.T) {
	s := New()
	if s.ID() != "himeros" {
		t.Errorf("ID() = %q", s.ID())
	}
	for _, u := range []string{
		"https://himeros.tv/",
		"https://himeros.tv/tour/",
		"https://www.himeros.tv/tour/trailers/The-Good-Boys.html",
	} {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	for _, u := range []string{"https://himeros.com/", "https://example.com/himeros.tv"} {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}
