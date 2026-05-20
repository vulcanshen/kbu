package ui

import "testing"

func TestCountLines(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{"empty", "", 0},
		{"single no newline", "hello", 1},
		{"single trailing newline", "hello\n", 1},
		{"two lines no trailing", "a\nb", 2},
		{"two lines trailing", "a\nb\n", 2},
		{"blank line counts", "a\n\nb", 3},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := countLines(tc.in); got != tc.want {
				t.Errorf("countLines(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}
