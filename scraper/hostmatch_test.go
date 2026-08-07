package scraper

import "testing"

func TestHostMatches(t *testing.T) {
	tests := []struct {
		url   string
		hosts []string
		want  bool
	}{
		{"https://example.com", []string{"example.com"}, true},
		{"https://example.com/", []string{"example.com"}, true},
		{"https://www.example.com/videos", []string{"example.com"}, true},
		{"http://example.com/a/b?c=d#e", []string{"example.com"}, true},
		{"https://EXAMPLE.com", []string{"example.com"}, true},
		{"https://example.com", []string{"www.example.com"}, true},
		{"https://example.com:8080/x", []string{"example.com"}, true},
		{"https://tour.example.com", []string{"tour.example.com", "example.com"}, true},
		{"https://b.com", []string{"a.com", "b.com"}, true},

		// The whole point: a host that merely contains the target.
		{"https://example.com.evil.invalid/", []string{"example.com"}, false},
		{"https://notexample.com/", []string{"example.com"}, false},
		{"https://example.com.br/", []string{"example.com"}, false},
		{"https://evil.invalid/?x=https://example.com", []string{"example.com"}, false},
		{"https://evil.invalid/example.com", []string{"example.com"}, false},

		// Subdomains are not implied.
		{"https://tour.example.com", []string{"example.com"}, false},

		{"", []string{"example.com"}, false},
		{"not a url", []string{"example.com"}, false},
		{"https://example.com", nil, false},
		{"https://example.com", []string{""}, false},
	}

	for _, tt := range tests {
		if got := HostMatches(tt.url, tt.hosts...); got != tt.want {
			t.Errorf("HostMatches(%q, %v) = %v, want %v", tt.url, tt.hosts, got, tt.want)
		}
	}
}
