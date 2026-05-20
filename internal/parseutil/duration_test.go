package parseutil

import "testing"

func TestParseDurationColon(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"30:00", 1800},
		{"1:30", 90},
		{"00:28:51", 1731},
		{"01:00:00", 3600},
		{"1:02:03", 3723},
		{"45", 45},
		{"", 0},
		{"  30:00  ", 1800},
		{" 10 : 20 ", 620},
	}
	for _, tt := range tests {
		if got := ParseDurationColon(tt.in); got != tt.want {
			t.Errorf("ParseDurationColon(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestParseDurationISO(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"PT1H2M3S", 3723},
		{"PT30M", 1800},
		{"PT45S", 45},
		{"PT1H", 3600},
		{"PT1H30S", 3630},
		{"PT2M30S", 150},
		{"", 0},
		{"null", 0},
		{"  PT10M5S  ", 605},
	}
	for _, tt := range tests {
		if got := ParseDurationISO(tt.in); got != tt.want {
			t.Errorf("ParseDurationISO(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}
