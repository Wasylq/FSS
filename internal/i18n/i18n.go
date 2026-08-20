// Package i18n provides a minimal, stdlib-only lookup for translating the
// CLI's help text. Missing keys and empty values fall back to English.
package i18n

import (
	"embed"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
)

//go:embed locales/*.json
var localeFS embed.FS

// SourceLanguage is the language the source strings are written in. Its
// catalog, locales/en.json, is the monolingual base file the translation
// platform reads; it is never installed, so T is the identity for English.
const SourceLanguage = "en"

// state pairs a tag with its catalog so both swap atomically. A nil m means
// English (no lookups performed).
type state struct {
	tag string
	m   map[string]string
}

var active atomic.Pointer[state] // nil pointer == English; lock-free for -race

// T translates s through the active catalog. Missing key or empty value
// falls back to s itself.
func T(s string) string {
	st := active.Load()
	if st == nil || st.m == nil {
		return s
	}
	if v, ok := st.m[s]; ok && v != "" {
		return v
	}
	return s
}

var rawCandidateRe = regexp.MustCompile(`^[A-Za-z0-9_]{1,16}$`)

// SetLanguage installs the catalog for tag and returns the language actually
// installed (SourceLanguage if none matched).
func SetLanguage(tag string) string {
	raw := strings.TrimSpace(tag)
	norm := Normalize(raw)
	if norm == "" || norm == SourceLanguage {
		active.Store(&state{tag: SourceLanguage})
		return SourceLanguage
	}

	var candidates []string
	add := func(c string) {
		if c == "" {
			return
		}
		for _, existing := range candidates {
			if existing == c {
				return
			}
		}
		candidates = append(candidates, c)
	}
	if rawCandidateRe.MatchString(raw) {
		add(raw)
	}
	add(norm)
	if i := strings.IndexByte(norm, '_'); i >= 0 {
		add(norm[:i])
	}

	for _, c := range candidates {
		// locales/en.json exists as the base file, but English needs no
		// catalog: stop rather than install a map of every key onto itself.
		if c == SourceLanguage {
			break
		}
		data, err := localeFS.ReadFile("locales/" + c + ".json")
		if err != nil {
			continue
		}
		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		active.Store(&state{tag: c, m: m})
		return c
	}

	active.Store(&state{tag: SourceLanguage})
	return SourceLanguage
}

// Language returns the active tag, SourceLanguage if none was set.
func Language() string {
	st := active.Load()
	if st == nil {
		return SourceLanguage
	}
	return st.tag
}

// Available returns "en" followed by the sorted translation catalogs shipped
// under locales/. The "_"-prefixed generated files are skipped, as is
// en.json: it is the base file, not a translation of it.
func Available() []string {
	entries, err := localeFS.ReadDir("locales")
	if err != nil {
		return []string{SourceLanguage}
	}
	var tags []string
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".json")
		if strings.HasPrefix(name, "_") || name == SourceLanguage {
			continue
		}
		tags = append(tags, name)
	}
	sort.Strings(tags)
	return append([]string{SourceLanguage}, tags...)
}

// Normalize canonicalizes a locale tag (e.g. "ko_KR.UTF-8" or "ko-KR") into
// the form used for catalog filenames, or "" if it can't be trusted.
func Normalize(tag string) string {
	s := strings.TrimSpace(tag)
	if i := strings.IndexAny(s, ".@"); i >= 0 {
		s = s[:i]
	}
	s = strings.ReplaceAll(s, "-", "_")

	if s == "" || strings.EqualFold(s, "C") || strings.EqualFold(s, "POSIX") {
		return ""
	}
	if strings.ContainsAny(s, "/\\.") || len(s) > 16 {
		return ""
	}

	if i := strings.IndexByte(s, '_'); i >= 0 {
		return strings.ToLower(s[:i]) + "_" + strings.ToUpper(s[i+1:])
	}
	return strings.ToLower(s)
}
