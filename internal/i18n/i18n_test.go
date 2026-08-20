package i18n

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"codeset stripped", "ko_KR.UTF-8", "ko_KR"},
		{"bcp47 hyphen", "ko-KR", "ko_KR"},
		{"case folded", "KO_kr", "ko_KR"},
		{"C locale", "C", ""},
		{"POSIX locale", "POSIX", ""},
		{"empty", "", ""},
		{"path traversal", "../etc/passwd", ""},
		{"17 bytes", "abcdefghijklmnopq", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Normalize(tt.in); got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSetLanguage(t *testing.T) {
	t.Cleanup(func() { active.Store(nil) })

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", SourceLanguage},
		{"english", "en", SourceLanguage},
		{"unknown tag", "xx", SourceLanguage},
		{"exact catalog", "ko", "ko"},
		{"base-language fallback", "ko_KR", "ko"},
		{"ambient locale", "ko_KR.UTF-8", "ko"},
		{"underscore-prefixed catalog reachable by raw name", "_pseudo", "_pseudo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SetLanguage(tt.in); got != tt.want {
				t.Errorf("SetLanguage(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if got := Language(); got != tt.want {
				t.Errorf("Language() after SetLanguage(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// T's fallback paths need a catalog with a missing key and one with a
// present-but-empty value. No shipped locale exercises the latter, so this
// reaches into the package's own state directly rather than going through
// SetLanguage.
func TestT(t *testing.T) {
	t.Cleanup(func() { active.Store(nil) })

	active.Store(nil)
	if got := T("hello"); got != "hello" {
		t.Errorf("T with no state = %q, want %q", got, "hello")
	}

	active.Store(&state{tag: "xx", m: map[string]string{}})
	if got := T("hello"); got != "hello" {
		t.Errorf("T with missing key = %q, want %q", got, "hello")
	}

	active.Store(&state{tag: "xx", m: map[string]string{"hello": ""}})
	if got := T("hello"); got != "hello" {
		t.Errorf("T with empty value = %q, want %q", got, "hello")
	}

	active.Store(&state{tag: "xx", m: map[string]string{"hello": "안녕"}})
	if got := T("hello"); got != "안녕" {
		t.Errorf("T with translated value = %q, want %q", got, "안녕")
	}
}

// TestSetLanguageInstallsCatalog checks the installed catalog is actually
// consulted, not merely that the tag resolved.
func TestSetLanguageInstallsCatalog(t *testing.T) {
	t.Cleanup(func() { active.Store(nil) })

	if got := SetLanguage("ko_KR.UTF-8"); got != "ko" {
		t.Fatalf("SetLanguage = %q, want %q", got, "ko")
	}
	const key = "Usage:"
	if got := T(key); got == key {
		t.Errorf("T(%q) returned the English source; ko.json was not consulted", key)
	}
	if got := T("no such key"); got != "no such key" {
		t.Errorf("T on a missing key = %q, want passthrough", got)
	}
}

func TestAvailable(t *testing.T) {
	got := Available()
	if len(got) == 0 || got[0] != SourceLanguage {
		t.Fatalf("Available() = %v, want first element %q", got, SourceLanguage)
	}
	for _, tag := range got[1:] {
		if len(tag) > 0 && tag[0] == '_' {
			t.Errorf("Available() = %v, contains _-prefixed entry %q", got, tag)
		}
		// en.json is the base file: listing it would print "en, en, ko".
		if tag == SourceLanguage {
			t.Errorf("Available() = %v, lists the source language twice", got)
		}
	}
}
