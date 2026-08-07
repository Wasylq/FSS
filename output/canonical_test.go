package output

import "testing"

func TestCanonicalStudioURL(t *testing.T) {
	// Every spelling of one catalogue collapses to one key.
	same := []string{
		"http://www.example.com",
		"https://www.example.com",
		"https://www.example.com/",
		"https://WWW.Example.com/",
		"HTTP://WWW.EXAMPLE.COM/",
		"https://www.example.com:443/",
		"http://www.example.com:80",
		"https://www.example.com/#section",
		"  https://www.example.com/  ",
	}
	want := CanonicalStudioURL(same[0])
	if want != "https://www.example.com" {
		t.Fatalf("canonical form = %q, want https://www.example.com", want)
	}
	for _, u := range same[1:] {
		if got := CanonicalStudioURL(u); got != want {
			t.Errorf("CanonicalStudioURL(%q) = %q, want %q", u, got, want)
		}
	}

	// Genuinely different catalogues must stay different.
	distinct := [][2]string{
		{"https://example.com/studio/1", "https://example.com/studio/2"},
		{"https://example.com/Studio", "https://example.com/studio"}, // paths are case-sensitive
		{"https://a.example.com", "https://b.example.com"},
		{"https://example.com/x?page=1", "https://example.com/x?page=2"},
	}
	for _, p := range distinct {
		if CanonicalStudioURL(p[0]) == CanonicalStudioURL(p[1]) {
			t.Errorf("%q and %q collapsed together", p[0], p[1])
		}
	}

	// Unparseable input is returned untouched rather than mangled.
	for _, bad := range []string{"", "not a url", "://nope"} {
		if got := CanonicalStudioURL(bad); got != bad {
			t.Errorf("CanonicalStudioURL(%q) = %q, want it unchanged", bad, got)
		}
	}

	// Idempotent.
	for _, u := range same {
		once := CanonicalStudioURL(u)
		if twice := CanonicalStudioURL(once); twice != once {
			t.Errorf("not idempotent: %q -> %q -> %q", u, once, twice)
		}
	}
}
