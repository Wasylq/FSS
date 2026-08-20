package i18n

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const templateFile = "locales/en.json"

// catalogFiles returns every shipped catalog, "_"-prefixed ones included.
func catalogFiles(t *testing.T) []string {
	t.Helper()
	entries, err := localeFS.ReadDir("locales")
	if err != nil {
		t.Fatalf("reading locales: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, "locales/"+e.Name())
	}
	sort.Strings(names)
	return names
}

func readCatalog(t *testing.T, path string) map[string]string {
	t.Helper()
	data, err := localeFS.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return m
}

func TestCatalogsParse(t *testing.T) {
	for _, path := range catalogFiles(t) {
		if got := readCatalog(t, path); got == nil {
			t.Errorf("%s: decoded to a nil map", path)
		}
	}
}

// TestCatalogKeysAreSubsetOfTemplate catches a key left stale by an English
// reword — the failure mode where a translation silently stops applying.
func TestCatalogKeysAreSubsetOfTemplate(t *testing.T) {
	template := readCatalog(t, templateFile)
	for _, path := range catalogFiles(t) {
		name := strings.TrimPrefix(path, "locales/")
		if path == templateFile || strings.HasPrefix(name, "_") {
			continue
		}
		for key := range readCatalog(t, path) {
			if _, ok := template[key]; !ok {
				t.Errorf("%s: key not in the template:\n%q\n"+
					"re-run `make i18n-extract`, then update the catalog", path, key)
			}
		}
	}
}

var verbRe = regexp.MustCompile(`%[-+# 0-9.]*[a-zA-Z]`)

// verbs returns the fmt verbs in s, sorted, with %% escapes removed.
func verbs(s string) []string {
	found := verbRe.FindAllString(strings.ReplaceAll(s, "%%", ""), -1)
	sort.Strings(found)
	return found
}

// TestCatalogPlaceholderParity guards against %!s(MISSING) in a user's help.
func TestCatalogPlaceholderParity(t *testing.T) {
	for _, path := range catalogFiles(t) {
		for key, value := range readCatalog(t, path) {
			if value == "" {
				continue
			}
			want, got := verbs(key), verbs(value)
			if strings.Join(want, ",") != strings.Join(got, ",") {
				t.Errorf("%s: placeholder mismatch %v vs %v for key:\n%q", path, want, got, key)
			}
		}
	}
}

// TestCatalogsHaveNoTemplateDelimiters rejects at review time what safe()
// would otherwise have to strip at render time.
func TestCatalogsHaveNoTemplateDelimiters(t *testing.T) {
	for _, path := range catalogFiles(t) {
		for key, value := range readCatalog(t, path) {
			if strings.Contains(value, "{{") || strings.Contains(value, "}}") {
				t.Errorf("%s: value contains a template delimiter, for key:\n%q", path, key)
			}
		}
	}
}
