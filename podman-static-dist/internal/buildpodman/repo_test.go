package buildpodman

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestSyncCloneThenFetch drives both code paths against a local source repo so
// no network is needed: first a fresh clone at one tag, then a re-sync that must
// fetch a newly added tag into the existing work tree.
func TestSyncCloneThenFetch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	ctx := context.Background()
	src := t.TempDir()
	gitInit(t, src)
	writeCommit(t, src, "a.txt", "one", "v0.0.1")

	dest := filepath.Join(t.TempDir(), "checkout")
	if err := Sync(ctx, dest, src, "v0.0.1"); err != nil {
		t.Fatalf("clone sync: %v", err)
	}
	if got := readFile(t, filepath.Join(dest, "a.txt")); got != "one" {
		t.Fatalf("a.txt = %q after v0.0.1", got)
	}

	// New tag in the source; re-sync must fetch and check it out in place.
	writeCommit(t, src, "a.txt", "two", "v0.0.2")
	if err := Sync(ctx, dest, src, "v0.0.2"); err != nil {
		t.Fatalf("fetch sync: %v", err)
	}
	if got := readFile(t, filepath.Join(dest, "a.txt")); got != "two" {
		t.Fatalf("a.txt = %q after v0.0.2", got)
	}
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init", "-q", "-b", "main")
}

func writeCommit(t *testing.T, dir, name, content, tag string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", name)
	runGit(t, dir,
		"-c", "user.email=t@t", "-c", "user.name=t",
		"commit", "-q", "-m", tag)
	runGit(t, dir, "tag", tag)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
