package parseutil

import "testing"

func FuzzParseDurationColon(f *testing.F) {
	f.Add("12:34")
	f.Add("1:02:03")
	f.Add("0:00")
	f.Add("")
	f.Add("abc:def")
	f.Add("99:99:99")

	f.Fuzz(func(t *testing.T, s string) {
		n := ParseDurationColon(s)
		if n < 0 {
			t.Errorf("ParseDurationColon(%q) = %d, must be non-negative", s, n)
		}
	})
}

func FuzzParseDurationISO(f *testing.F) {
	f.Add("PT1H2M3S")
	f.Add("PT30M")
	f.Add("PT45S")
	f.Add("")
	f.Add("null")
	f.Add("garbage")
	f.Add("PT999H999M999S")

	f.Fuzz(func(t *testing.T, s string) {
		n := ParseDurationISO(s)
		if n < 0 {
			t.Errorf("ParseDurationISO(%q) = %d, must be non-negative", s, n)
		}
	})
}
