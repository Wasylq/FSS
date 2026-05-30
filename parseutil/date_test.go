package parseutil

import (
	"testing"
	"time"
)

func TestTryParseDate_firstLayoutWins(t *testing.T) {
	got, err := TryParseDate("2024-06-15", "2006-01-02", time.RFC3339)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestTryParseDate_fallsThrough(t *testing.T) {
	got, err := TryParseDate("Jun 15, 2024", "2006-01-02", "Jan 2, 2006")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestTryParseDate_noMatch(t *testing.T) {
	_, err := TryParseDate("not-a-date", "2006-01-02", time.RFC3339)
	if err == nil {
		t.Error("expected error for unrecognized date")
	}
}

func TestTryParseDate_emptyLayouts(t *testing.T) {
	_, err := TryParseDate("2024-06-15")
	if err == nil {
		t.Error("expected error with no layouts")
	}
}

func TestTryParseDate_rfc3339Variants(t *testing.T) {
	layouts := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05"}
	tests := []struct {
		input string
		want  time.Time
	}{
		{"2024-06-15T10:30:00Z", time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)},
		{"2024-06-15T10:30:00.123456789Z", time.Date(2024, 6, 15, 10, 30, 0, 123456789, time.UTC)},
		{"2024-06-15T10:30:00", time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		got, err := TryParseDate(tt.input, layouts...)
		if err != nil {
			t.Errorf("TryParseDate(%q): %v", tt.input, err)
			continue
		}
		if !got.Equal(tt.want) {
			t.Errorf("TryParseDate(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestTryParseDate_longMonthVariants(t *testing.T) {
	layouts := []string{"January 2, 2006", "Jan 2, 2006", "January 2 2006", "Jan 2 2006"}
	tests := []struct {
		input string
		want  time.Time
	}{
		{"June 15, 2024", time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)},
		{"Jun 15, 2024", time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)},
		{"June 15 2024", time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)},
		{"Jun 15 2024", time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		got, err := TryParseDate(tt.input, layouts...)
		if err != nil {
			t.Errorf("TryParseDate(%q): %v", tt.input, err)
			continue
		}
		if !got.Equal(tt.want) {
			t.Errorf("TryParseDate(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
