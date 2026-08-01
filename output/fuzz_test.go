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
	// Regression: a URL long enough to push the slug past the 255-byte filename
	// limit made the Flat store fail with ENAMETOOLONG on Save, after the whole
	// scrape had already run.
	f.Add("https://example.com/" + strings.Repeat("a", 500))
	f.Add(strings.Repeat("h", 4000))

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

		// The slug becomes a filename component: the Flat store writes
		// "<slug>.json" and flocks "<slug>.lock". Filenames are capped at 255
		// bytes, so an unbounded slug is not a cosmetic problem — Save fails
		// after the scrape has finished and the run is lost.
		if len(slug) > maxSlugLen {
			t.Errorf("Slugify(%q) returned %d bytes, over the %d-byte cap; "+
				"<slug>.json would exceed the 255-byte filename limit", rawURL, len(slug), maxSlugLen)
		}

		// Never empty, even for input that sanitizes away entirely: an empty
		// stem makes every such studio share the file ".json" and overwrite each
		// other, which is silent data loss rather than an error.
		if slug == "" {
			t.Errorf("Slugify(%q) returned an empty stem", rawURL)
		}

		// Deterministic — the slug *is* the store key, so an unstable one
		// orphans the previous file and restarts the scrape from empty.
		if again := Slugify(rawURL); again != slug {
			t.Errorf("Slugify(%q) is not deterministic: %q then %q", rawURL, slug, again)
		}
	})
}
