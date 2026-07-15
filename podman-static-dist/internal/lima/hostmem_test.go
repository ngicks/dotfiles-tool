package lima

import "testing"

func TestCapMemory(t *testing.T) {
	const giB = 1 << 30
	for _, tt := range []struct {
		name string
		host uint64
		want string
	}{
		{"large host capped at 8GiB", 32 * giB, "8192MiB"},
		{"half equals cap", 16 * giB, "8192MiB"},
		{"small host halved", 8 * giB, "4096MiB"},
		{"tiny host halved", 4 * giB, "2048MiB"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := capMemory(tt.host); got != tt.want {
				t.Errorf("capMemory(%d) = %q, want %q", tt.host, got, tt.want)
			}
		})
	}
}
