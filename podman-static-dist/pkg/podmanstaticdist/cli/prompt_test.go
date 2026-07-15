package cli

import (
	"strings"
	"testing"
)

func TestConfirm(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"y\n", true},
		{"Y\n", true},
		{"yes\n", true},
		{"n\n", false},
		{"\n", false},
		{"", false},
		{"nope\n", false},
	}
	for _, tt := range tests {
		var out strings.Builder
		got, err := Confirm(strings.NewReader(tt.in), &out, "Proceed?")
		if err != nil {
			t.Fatalf("in=%q: %v", tt.in, err)
		}
		if got != tt.want {
			t.Errorf("Confirm(%q) = %v, want %v", tt.in, got, tt.want)
		}
		if !strings.Contains(out.String(), "Proceed? [y/N]:") {
			t.Errorf("prompt not written: %q", out.String())
		}
	}
}
