package cmd

import "testing"

func TestNormalizeInputURL(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		// Already has a scheme — untouched, including http-only sites.
		{"https://www.21sextury.com/", "https://www.21sextury.com/"},
		{"http://21sextury.com", "http://21sextury.com"},

		// Bare hosts: the case that used to report "Not supported".
		{"www.21sextury.com", "https://www.21sextury.com"},
		{"21sextury.com/", "https://21sextury.com/"},
		{"21sextury.com", "https://21sextury.com"},
		{"www.21sextury.com/en/videos", "https://www.21sextury.com/en/videos"},

		{"  21sextury.com  ", "https://21sextury.com"},
		{"", ""},

		// A "://" inside the path is not a scheme.
		{"example.com/redirect?to=https://other.com", "https://example.com/redirect?to=https://other.com"},
	}

	for _, tt := range tests {
		if got := normalizeInputURL(tt.in); got != tt.want {
			t.Errorf("normalizeInputURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
