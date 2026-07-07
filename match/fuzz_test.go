package match

import (
	"testing"
	"unicode"
)

func FuzzNormalize(f *testing.F) {
	f.Add("Fostering the Bully")
	f.Add("MILF JOI Countdown!!!")
	f.Add("Scene - Title (4K)")
	f.Add("camelCaseWords")
	f.Add("")
	f.Add("Hello\u200bWorld")
	f.Add("Café Scene")
	f.Add("Привет Мир") // Cyrillic — must survive, not be stripped to ASCII
	f.Add("日本語のタイトル")   // CJK
	f.Add("ﺡ")          // Arabic presentation form (regression: kept as a letter)
	f.Add("\xf7û")      // invalid UTF-8 + accented letter (idempotency regression)

	f.Fuzz(func(t *testing.T, s string) {
		out := Normalize(s)

		// Idempotent: normalizing again should not change the result.
		if out2 := Normalize(out); out2 != out {
			t.Errorf("not idempotent: Normalize(%q) = %q, Normalize(%q) = %q", s, out, out, out2)
		}

		// Normalize preserves letters and digits of every script (Cyrillic, CJK,
		// Arabic, …), so the invariant is Unicode-aware rather than ASCII-only:
		// output holds only lowercase letters, digits, and single spaces.
		for _, r := range out {
			switch {
			case r == ' ', unicode.IsDigit(r):
				// allowed
			case unicode.IsLetter(r):
				if r != unicode.ToLower(r) {
					t.Errorf("Normalize(%q) = %q contains non-lowercase rune %q", s, out, r)
					return
				}
			default:
				t.Errorf("Normalize(%q) = %q contains invalid rune %q", s, out, r)
				return
			}
		}
	})
}
