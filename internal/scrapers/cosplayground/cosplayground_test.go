package cosplayground

import "testing"

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://cosplayground.com/", true},
		{"https://www.cosplayground.com/", true},
		{"http://cosplayground.com/tour/whats-new", true},
		{"https://cosplayground.com/tour/trailer/some-slug/", true},
		// Prefix trap — must not match a look-alike host.
		{"https://cosplaygroundfake.com/", false},
		{"https://example.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestID(t *testing.T) {
	if got := New().ID(); got != "cosplayground" {
		t.Errorf("ID() = %q, want cosplayground", got)
	}
}
