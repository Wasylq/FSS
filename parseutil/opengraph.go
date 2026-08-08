package parseutil

import "regexp"

// ogPropFirstRe matches `<meta property="og:X" content="Y">`.
// ogContentFirstRe matches the same tag with attributes reversed —
// `<meta content="Y" property="og:X">`. Real sites use both orderings;
// running both and merging covers each.
// RE2 has no backreferences, so each quoting style is its own alternative and
// the value arrives in whichever group matched. `[^>]*?` lets unrelated
// attributes sit between the two without escaping the tag.
var (
	ogPropFirstRe    = regexp.MustCompile(`(?i)<meta\s+property=(?:"(og:[^"]+)"|'(og:[^']+)')\s+[^>]*?content=(?:"([^"]*)"|'([^']*)')`)
	ogContentFirstRe = regexp.MustCompile(`(?i)<meta\s+content=(?:"([^"]*)"|'([^']*)')\s+[^>]*?property=(?:"(og:[^"]+)"|'(og:[^']+)')`)
)

// firstNonEmpty returns the first non-empty submatch, for the quote-style
// alternatives above.
func firstNonEmpty(groups ...[]byte) string {
	for _, g := range groups {
		if len(g) > 0 {
			return string(g)
		}
	}
	return ""
}

// OpenGraph extracts every `<meta property="og:*" content="…">` pair
// from `body` into a map keyed by the full property name (e.g.
// `"og:title"`, `"og:image"`, `"og:video:duration"`). Both attribute
// orderings (`property` then `content`, and the reverse) are
// recognised; this matches what real sites emit in practice.
//
// The returned map values are the raw `content` attribute strings as
// they appear in the source — HTML entities are NOT decoded, and
// trailing whitespace is preserved. Callers that need an unescaped
// string should pass the value through `html.UnescapeString` (and
// `strings.TrimSpace` where relevant). The values are kept raw so the
// helper matches what scrapers were doing before extraction without
// taking on entity-handling decisions they each made differently.
//
// Repeated `og:foo` tags (e.g. multiple `og:image` entries on
// articles) collapse to the last occurrence in source order. If a
// caller needs every value, switch to FindAllStringSubmatch directly.
//
// Returns an empty (non-nil) map if no OpenGraph tags are present.
func OpenGraph(body []byte) map[string]string {
	out := make(map[string]string)
	for _, m := range ogPropFirstRe.FindAllSubmatch(body, -1) {
		out[firstNonEmpty(m[1], m[2])] = firstNonEmpty(m[3], m[4])
	}
	for _, m := range ogContentFirstRe.FindAllSubmatch(body, -1) {
		out[firstNonEmpty(m[3], m[4])] = firstNonEmpty(m[1], m[2])
	}
	return out
}
