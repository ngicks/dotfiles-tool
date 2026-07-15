package install

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
)

func writeTreeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func linkFixture(t *testing.T) (base string, env Env) {
	t.Helper()
	root := t.TempDir()
	base = filepath.Join(root, "base")
	home := filepath.Join(root, "home")
	tagDir := filepath.Join(base, "v1")

	writeTreeFile(t, filepath.Join(tagDir, "usr/local/bin/podman"), "binary\n")
	writeTreeFile(
		t,
		filepath.Join(tagDir, "etc/environment.d/50-podman.conf"),
		"PODMAN=1\n",
	)
	writeTreeFile(
		t,
		filepath.Join(tagDir, "usr/local/lib/systemd/user/podman.socket"),
		"[Socket]\n",
	)
	writeTreeFile(t, filepath.Join(tagDir, "etc/containers/storage.conf"), "[storage.options]\n")
	writeTreeFile(t, filepath.Join(tagDir, "etc/containers/registries.conf"), "unqualified = []\n")

	if err := os.Symlink("v1", filepath.Join(base, "current")); err != nil {
		t.Fatal(err)
	}

	env = Env{
		Home:       home,
		DataHome:   filepath.Join(home, ".local/share"),
		ConfigHome: filepath.Join(home, ".config"),
	}
	return base, env
}

// assertWired checks the link contract: ~/.config/containers is a wholesale
// symlink into the current tree (interpolation happens at extract time, not
// here), plus the per-file environment.d link. Binaries have no home-side
// link: consumers address <base>/current/usr/local directly.
func assertWired(t *testing.T, base string, env Env) {
	t.Helper()
	current := filepath.Join(base, "current")
	configDir := filepath.Join(env.ConfigHome, "containers")

	if _, err := os.Lstat(filepath.Join(env.Home, ".local/containers")); !errors.Is(
		err, fs.ErrNotExist,
	) {
		t.Errorf("~/.local/containers staging link should no longer be created (err %v)", err)
	}

	if got, err := os.Readlink(configDir); err != nil ||
		got != filepath.Join(current, "etc/containers") {
		t.Errorf("~/.config/containers link = %q (err %v), want %s",
			got, err, filepath.Join(current, "etc/containers"))
	}

	want := filepath.Join(current, "etc/environment.d/50-podman.conf")
	envLink := filepath.Join(env.ConfigHome, "environment.d/50-podman.conf")
	if got, err := os.Readlink(envLink); err != nil || got != want {
		t.Errorf("environment.d link = %q (err %v), want %s", got, err, want)
	}
}

// TestLinkHappyPath is the canonical Link contract: it wholesale-symlinks
// ~/.config/containers into the current tree (no per-file materialization).
func TestLinkHappyPath(t *testing.T) {
	base, env := linkFixture(t)
	o := LinkOption{
		Base:        base,
		SkipSystemd: true,
		Env:         env,
	}
	if err := Link(context.Background(), o); err != nil {
		t.Fatalf("Link: %v", err)
	}
	assertWired(t, base, env)
}

func TestLinkIdempotent(t *testing.T) {
	base, env := linkFixture(t)
	o := LinkOption{
		Base:        base,
		SkipSystemd: true,
		Env:         env,
	}
	if err := Link(context.Background(), o); err != nil {
		t.Fatalf("first Link: %v", err)
	}
	if err := Link(context.Background(), o); err != nil {
		t.Fatalf("second Link: %v", err)
	}
	assertWired(t, base, env)
}

// TestLinkReplacesStaleConfigSymlink verifies a prior ~/.config/containers
// symlink pointing at a stale target is repointed at the current tree.
func TestLinkReplacesStaleConfigSymlink(t *testing.T) {
	base, env := linkFixture(t)
	configDir := filepath.Join(env.ConfigHome, "containers")

	if err := os.MkdirAll(filepath.Dir(configDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(
		filepath.Join(base, "stale/etc/containers"),
		configDir,
	); err != nil {
		t.Fatal(err)
	}

	o := LinkOption{
		Base:        base,
		SkipSystemd: true,
		Env:         env,
	}
	if err := Link(context.Background(), o); err != nil {
		t.Fatalf("Link: %v", err)
	}
	assertWired(t, base, env)
}

func TestLinkReadOnlyBaseTolerated(t *testing.T) {
	base, env := linkFixture(t)
	current := filepath.Join(base, "current")

	if err := os.Chmod(base, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(base, 0o755) })

	probe := filepath.Join(base, ".probe")
	if f, err := os.Create(probe); err == nil {
		_ = f.Close()
		_ = os.Remove(probe)
		t.Skip("base writable despite chmod 0555 (running as root?); " +
			"read-only tolerance branch not exercised")
	}

	o := LinkOption{
		Base:        base,
		Tag:         "v2",
		SkipSystemd: true,
		Env:         env,
	}
	if err := Link(context.Background(), o); err != nil {
		t.Fatalf("read-only base with a valid current should be tolerated: %v", err)
	}
	if got, err := os.Readlink(current); err != nil || got != "v1" {
		t.Errorf("current = %q (err %v), want unchanged v1", got, err)
	}
	assertWired(t, base, env)
}

func TestLinkMissingCurrentNoTag(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "base")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "home")
	o := LinkOption{
		Base:        base,
		SkipSystemd: true,
		Env: Env{
			Home:       home,
			DataHome:   filepath.Join(home, ".local/share"),
			ConfigHome: filepath.Join(home, ".config"),
		},
	}
	err := Link(context.Background(), o)
	if err == nil {
		t.Fatal("expected an error when current is missing and --tag is unset")
	}
	if !strings.Contains(err.Error(), "no current symlink") {
		t.Errorf("error = %v, want it to mention the missing current symlink", err)
	}
}

func TestLinkSkipSystemd(t *testing.T) {
	base, env := linkFixture(t)
	o := LinkOption{
		Base:        base,
		SkipSystemd: true,
		Env:         env,
	}
	if err := Link(context.Background(), o); err != nil {
		t.Fatalf("Link: %v", err)
	}
	if _, err := os.Lstat(
		filepath.Join(env.ConfigHome, "environment.d/50-podman.conf"),
	); err != nil {
		t.Errorf("environment.d link missing under --skip-systemd: %v", err)
	}
	if _, err := os.Lstat(
		filepath.Join(env.ConfigHome, "systemd/user/podman.socket"),
	); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("systemd user link created despite --skip-systemd (err %v)", err)
	}
}

func TestDirLinkTableAndApply(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(src, "50-podman.conf"),
		[]byte("X=1\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	linkDir := filepath.Join(root, "cfg/environment.d")
	if err := os.MkdirAll(linkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(linkDir, "75-other.conf")
	if err := os.WriteFile(other, []byte("Y=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var rules []linkRule
	for name, err := range listDirent(src, func(e os.DirEntry) bool { return e.Type().IsRegular() }) {
		if err != nil {
			t.Fatal(err)
		}
		rules = append(rules, linkRule{
			filepath.Join(linkDir, name),
			filepath.Join("/base/lib/environment.d", name),
		})
	}
	if err := applyLinks(rules); err != nil {
		t.Fatal(err)
	}
	got, err := os.Readlink(filepath.Join(linkDir, "50-podman.conf"))
	if err != nil {
		t.Fatalf("expected a symlink: %v", err)
	}
	if got != "/base/lib/environment.d/50-podman.conf" {
		t.Errorf("link target = %q", got)
	}
	if _, err := os.Lstat(other); err != nil {
		t.Errorf("unrelated fragment was removed: %v", err)
	}

	var missing []linkRule
	for name, err := range listDirent(
		filepath.Join(root, "absent"),
		func(e os.DirEntry) bool { return e.Type().IsRegular() },
	) {
		if err != nil {
			t.Fatalf("missing srcDir should be a no-op, got %v", err)
		}
		missing = append(missing, linkRule{filepath.Join(linkDir, name), "/base"})
	}
	if len(missing) != 0 {
		t.Errorf("missing srcDir produced %d rules, want 0", len(missing))
	}
}

func TestWiringRulesTable(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "base")
	current := filepath.Join(base, "current")
	home := filepath.Join(root, "home")

	writeTreeFile(
		t,
		filepath.Join(current, "etc/environment.d/50-podman.conf"),
		"E=1\n",
	)
	writeTreeFile(
		t,
		filepath.Join(current, "usr/local/lib/systemd/user/podman.socket"),
		"[Socket]\n",
	)

	env := Env{
		Home:       home,
		DataHome:   filepath.Join(home, ".local/share"),
		ConfigHome: filepath.Join(home, ".config"),
	}

	rules, err := wiringRules(wiringParams{
		env:     env,
		current: current,
		systemd: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	assertRow := func(label string, want linkRule) {
		if !slices.Contains(rules, want) {
			t.Errorf("%s pair %v not in table %v", label, want, rules)
		}
	}

	assertRow("environment.d", linkRule{
		filepath.Join(env.ConfigHome, "environment.d/50-podman.conf"),
		filepath.Join(current, "etc/environment.d/50-podman.conf"),
	})
	assertRow("systemd", linkRule{
		filepath.Join(env.ConfigHome, "systemd/user/podman.socket"),
		filepath.Join(current, "usr/local/lib/systemd/user/podman.socket"),
	})
	for _, r := range rules {
		if r[0] == filepath.Join(home, ".local/containers") {
			t.Errorf("staging-link row %v still emitted", r)
		}
	}
	// A nil embedded (wholesale mode) emits the single config-dir symlink.
	assertRow("config", linkRule{
		filepath.Join(env.ConfigHome, "containers"),
		filepath.Join(current, "etc/containers"),
	})
}

func TestApplyLinksRefusesNonSymlink(t *testing.T) {
	assertRefusal := func(t *testing.T, err error, path string) {
		t.Helper()
		if err == nil {
			t.Fatal("expected a refusal error, got nil")
		}
		if !strings.Contains(err.Error(), "refus") || !strings.Contains(err.Error(), path) {
			t.Errorf("error = %v, want it to mention the refusal and %s", err, path)
		}
	}

	t.Run("regular file via applyLinks", func(t *testing.T) {
		dest := filepath.Join(t.TempDir(), "conf")
		if err := os.WriteFile(dest, []byte("real\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		err := applyLinks([]linkRule{{dest, "/some/target"}})
		assertRefusal(t, err, dest)
		if b, err := os.ReadFile(dest); err != nil || string(b) != "real\n" {
			t.Errorf("real file disturbed: %q (err %v)", b, err)
		}
	})

	t.Run("directory via updateSymlink", func(t *testing.T) {
		dest := filepath.Join(t.TempDir(), "confdir")
		if err := os.Mkdir(dest, 0o755); err != nil {
			t.Fatal(err)
		}
		err := updateSymlink("/some/target", dest)
		assertRefusal(t, err, dest)
		if fi, err := os.Lstat(dest); err != nil || !fi.IsDir() {
			t.Errorf("real directory disturbed (err %v)", err)
		}
	})
}

func inodeOf(t *testing.T, path string) uint64 {
	t.Helper()
	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("no syscall.Stat_t for %s", path)
	}
	return st.Ino
}

func TestUpdateSymlinkSkipsWhenCorrect(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	before := inodeOf(t, link)
	if err := updateSymlink(target, link); err != nil {
		t.Fatalf("updateSymlink: %v", err)
	}
	if after := inodeOf(t, link); after != before {
		t.Errorf("inode changed %d -> %d, want the correct link left untouched", before, after)
	}
}

func TestUpdateSymlinkReplacesWrongTarget(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "link")
	if err := os.Symlink(filepath.Join(dir, "old"), link); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "new")
	if err := updateSymlink(want, link); err != nil {
		t.Fatalf("updateSymlink: %v", err)
	}
	if got, err := os.Readlink(link); err != nil || got != want {
		t.Errorf("link target = %q (err %v), want %s", got, err, want)
	}
}

func TestForceSymlinkSkipsWhenCorrect(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	before := inodeOf(t, link)
	if err := forceSymlink(target, link); err != nil {
		t.Fatalf("forceSymlink: %v", err)
	}
	if after := inodeOf(t, link); after != before {
		t.Errorf("inode changed %d -> %d, want the correct link left untouched", before, after)
	}
}

func TestForceSymlinkReplacesWrongTarget(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "link")
	if err := os.Symlink(filepath.Join(dir, "old"), link); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "new")
	if err := forceSymlink(want, link); err != nil {
		t.Fatalf("forceSymlink: %v", err)
	}
	if got, err := os.Readlink(link); err != nil || got != want {
		t.Errorf("link target = %q (err %v), want %s", got, err, want)
	}
}

func TestNeedElevateRootFastPath(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("root fast path only exercised when tests run as euid 0")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if needElevate(dir) {
		t.Errorf("needElevate(%q) = true for euid 0, want false", dir)
	}
}
