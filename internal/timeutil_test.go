package internal

import (
	"fmt"
	"testing"
	"time"
)

func TestFormatUptime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"zero", 0, "0s"},
		{"seconds only", 45 * time.Second, "45s"},
		{"minutes and seconds", 3*time.Minute + 12*time.Second, "3m12s"},
		{"hours and minutes", 2*time.Hour + 34*time.Minute, "2h34m"},
		{"days and hours", 3*24*time.Hour + 5*time.Hour, "3d5h"},
		{"negative clamped to zero", -10 * time.Second, "0s"},
		{"one second", 1 * time.Second, "1s"},
		{"one minute exactly", 1 * time.Minute, "1m0s"},
		{"one hour exactly", 1 * time.Hour, "1h0m"},
		{"one day exactly", 24 * time.Hour, "1d0h"},
		{"rounds sub-second up", 500 * time.Millisecond, "1s"},
		{"rounds sub-second down", 499 * time.Millisecond, "0s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatUptime(tt.duration)
			if got != tt.want {
				t.Fatalf("FormatUptime(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

func TestParseSystemdUSec(t *testing.T) {
	t.Parallel()

	t.Run("valid microsecond timestamp", func(t *testing.T) {
		t.Parallel()
		// 1_000_000 microseconds = 1 second after epoch
		got, err := ParseSystemdUSec("1000000")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := time.UnixMicro(1000000)
		if !got.Equal(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("real world timestamp", func(t *testing.T) {
		t.Parallel()
		// 2026-03-11T14:22:00Z in microseconds
		expected := time.Date(2026, 3, 11, 14, 22, 0, 0, time.UTC)
		usec := expected.UnixMicro()
		got, err := ParseSystemdUSec(fmt.Sprintf("%d", usec))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got.Equal(expected) {
			t.Fatalf("got %v, want %v", got, expected)
		}
	})

	t.Run("whitespace is trimmed", func(t *testing.T) {
		t.Parallel()
		got, err := ParseSystemdUSec("  1000000  ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := time.UnixMicro(1000000)
		if !got.Equal(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	errorCases := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"zero", "0"},
		{"n/a", "n/a"},
		{"N/A", "N/A"},
		{"negative", "-1000000"},
		{"not a number", "abc123"},
	}

	for _, tt := range errorCases {
		t.Run("error: "+tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseSystemdUSec(tt.input)
			if err == nil {
				t.Fatalf("expected error for input %q", tt.input)
			}
		})
	}
}

func TestParseSystemdTimestamp(t *testing.T) {
	t.Parallel()

	t.Run("RFC3339", func(t *testing.T) {
		t.Parallel()
		got, err := ParseSystemdTimestamp("2026-03-11T14:22:00Z")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := time.Date(2026, 3, 11, 14, 22, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("systemd format with timezone", func(t *testing.T) {
		t.Parallel()
		_, err := ParseSystemdTimestamp("Mon 2026-03-11 14:22:00 UTC")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	errorCases := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"n/a", "n/a"},
		{"garbage", "not-a-timestamp"},
	}

	for _, tt := range errorCases {
		t.Run("error: "+tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseSystemdTimestamp(tt.input)
			if err == nil {
				t.Fatalf("expected error for input %q", tt.input)
			}
		})
	}
}
