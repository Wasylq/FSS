package output

import (
	"strings"
	"testing"
)

func FuzzSlugify(f *testing.F) {
	f.Add("https://www.manyvids.com/Profile/590705/bettie-bondage/Store/Videos")
	f.Add("https://evil.com/../../etc/passwd")
	f.Add("")
	f.Add("https://example.com/a?b=c&d=e")
	f.Add("HTTPS://UPPER.COM/PATH")

	f.Fuzz(func(t *testing.T, rawURL string) {
		slug := Slugify(rawURL)

		if strings.Contains(slug, "..") {
			t.Errorf("Slugify(%q) = %q contains path traversal", rawURL, slug)
		}
		if strings.Contains(slug, "/") {
			t.Errorf("Slugify(%q) = %q contains slash", rawURL, slug)
		}
		if strings.HasPrefix(slug, "-") {
			t.Errorf("Slugify(%q) = %q starts with dash", rawURL, slug)
		}
		if strings.HasSuffix(slug, "-") {
			t.Errorf("Slugify(%q) = %q ends with dash", rawURL, slug)
		}
	})
}
