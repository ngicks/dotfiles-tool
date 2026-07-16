package rc

import (
	"io/fs"
	"testing"
)

// The embed directive silently drops files whose names start with "_" or "."
// unless the pattern carries the all: prefix; this guards the __additional_*
// config sets.
func TestFSIncludesUnderscorePrefixedDirs(t *testing.T) {
	for _, name := range []string{
		"etc/containers/__additional_podman-in-podman/containers.conf",
		"etc/containers/__additional_podman-in-podman/storage.conf",
		"etc/containers/__additional_podman-in-podman/path.sh",
	} {
		if _, err := fs.Stat(FS(), name); err != nil {
			t.Errorf("missing from embedded resource tree: %s: %v", name, err)
		}
	}
}
