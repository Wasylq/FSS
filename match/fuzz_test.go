package match

import "testing"

func FuzzNormalize(f *testing.F) {
	f.Add("Fostering the Bully")
	f.Add("MILF JOI Countdown!!!")
	f.Add("Scene - Title (4K)")
	f.Add("camelCaseWords")
	f.Add("")
	f.Add("Hello\u200bWorld")
	f.Add("Café Scene")

	f.Fuzz(func(t *testing.T, s string) {
		out := Normalize(s)

		// Idempotent: normalizing again should not change the result.
		if out2 := Normalize(out); out2 != out {
			t.Errorf("not idempotent: Normalize(%q) = %q, Normalize(%q) = %q", s, out, out, out2)
		}

		// Output must be lowercase ASCII alphanumeric + spaces only.
		for _, r := range out {
			ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' '
			if !ok {
				t.Errorf("Normalize(%q) = %q contains invalid rune %q", s, out, r)
				break
			}
		}
	})
}
