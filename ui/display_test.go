package ui

import (
	"testing"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{-1, "   —   "},
		{-1000, "   —   "},
		{0, "0 B"},
		{1, "1 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{1024 * 1024 * 1024 * 1024, "1.0 TB"},
	}
	for _, tt := range tests {
		if got := FormatSize(tt.bytes); got != tt.want {
			t.Errorf("FormatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestFormatComma(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{9, "9"},
		{999, "999"},
		{1000, "1,000"},
		{12345, "12,345"},
		{1234567, "1,234,567"},
		{1000000000, "1,000,000,000"},
	}
	for _, tt := range tests {
		if got := FormatComma(tt.n); got != tt.want {
			t.Errorf("FormatComma(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestSpinnerFrame(t *testing.T) {
	n := len(spinnerFrames)
	for i, want := range spinnerFrames {
		if got := SpinnerFrame(i); got != want {
			t.Errorf("SpinnerFrame(%d) = %q, want %q", i, got, want)
		}
	}
	// wraps around
	if SpinnerFrame(n) != SpinnerFrame(0) {
		t.Errorf("SpinnerFrame(%d) should wrap to SpinnerFrame(0)", n)
	}
	if SpinnerFrame(n+3) != SpinnerFrame(3) {
		t.Errorf("SpinnerFrame should be periodic with period %d", n)
	}
}

func TestTruncatePath(t *testing.T) {
	tests := []struct {
		path  string
		width int
		want  string
	}{
		{"/short", 20, "/short"},                        // fits, no truncation
		{"1234567890", 10, "1234567890"},                 // exactly width, no truncation
		{"/home/user/very/long/path", 10, "…long/path"}, // truncated: "…" + last 9 chars
		{"abcdef", 4, "…def"},                           // short width: "…" + last 3
	}
	for _, tt := range tests {
		if got := truncatePath(tt.path, tt.width); got != tt.want {
			t.Errorf("truncatePath(%q, %d) = %q, want %q", tt.path, tt.width, got, tt.want)
		}
	}
}
