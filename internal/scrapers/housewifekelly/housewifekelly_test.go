package housewifekelly

import "testing"

func TestIdentity(t *testing.T) {
	s := New()
	if s.ID() != "housewifekelly" {
		t.Errorf("ID() = %q", s.ID())
	}
	for _, u := range []string{
		"https://housewifekelly.com/",
		"https://www.housewifekelly.com/tour/",
		"https://www.housewifekelly.com/tour/categories/3some.html",
	} {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	// kelly4cash.com is the affiliate programme linked from every page, not a
	// catalogue.
	for _, u := range []string{"https://kelly4cash.com/", "https://tampahousewives.com/"} {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}
