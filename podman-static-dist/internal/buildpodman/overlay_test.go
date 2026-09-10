package buildpodman

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestOverlay(t *testing.T) {
	assetDir := t.TempDir()
	// The tree `make` produced: upstream etc/containers (plus much more we leave
	// untouched).
	writeFile(t, filepath.Join(assetDir, "etc/containers/containers.conf"), "# upstream\n")
	writeFile(t, filepath.Join(assetDir, "etc/containers/policy.json"), "{}\n")
	writeFile(
		t,
		filepath.Join(assetDir, "etc/containers/storage.conf.d/00-podman-static.conf"),
		"mount_program = \"/usr/local/bin/fuse-overlayfs\"\n",
	)
	writeFile(
		t,
		filepath.Join(assetDir, "etc/containers/containers.conf.d/00-x.conf"),
		"[engine]\n",
	)

	res := fstest.MapFS{
		"etc/containers/containers.conf":   {Data: []byte("conmon_path=[\"${HOME}/x\"]\n")},
		"etc/containers/storage.conf":      {Data: []byte("graphroot = ${XDG_DATA_HOME}/g\n")},
		"etc/environment.d/50-podman.conf": {Data: []byte("X=1\n")},
	}
	if err := Overlay(context.Background(), OverlayParams{
		AssetDir:   assetDir,
		ResourceFS: res,
	}); err != nil {
		t.Fatal(err)
	}

	// our conf overwrote upstream containers.conf, verbatim (placeholder intact).
	if got := readf(
		t,
		filepath.Join(assetDir, "etc/containers/containers.conf"),
	); got != "conmon_path=[\"${HOME}/x\"]\n" {
		t.Errorf("containers.conf = %q; want our overlay with placeholder intact", got)
	}
	// storage.conf added.
	if got := readf(
		t,
		filepath.Join(assetDir, "etc/containers/storage.conf"),
	); got != "graphroot = ${XDG_DATA_HOME}/g\n" {
		t.Errorf("storage.conf = %q", got)
	}
	// upstream policy.json we don't ship is left alone.
	if got := readf(t, filepath.Join(assetDir, "etc/containers/policy.json")); got != "{}\n" {
		t.Errorf("policy.json = %q; should be untouched", got)
	}
	// environment.d fragment delivered under etc/environment.d.
	if got := readf(
		t,
		filepath.Join(assetDir, "etc/environment.d/50-podman.conf"),
	); got != "X=1\n" {
		t.Errorf("environment.d = %q", got)
	}
	// upstream drop-in dirs are pruned so they cannot override our whole-file conf.
	for _, dir := range prunedDirs {
		if _, err := os.Stat(filepath.Join(assetDir, dir)); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s still present (err=%v); want pruned", dir, err)
		}
	}
}

func TestOverlayWithoutDropInDirs(t *testing.T) {
	assetDir := t.TempDir()
	writeFile(t, filepath.Join(assetDir, "etc/containers/containers.conf"), "# upstream\n")

	err := Overlay(context.Background(), OverlayParams{
		AssetDir: assetDir,
		ResourceFS: fstest.MapFS{
			"etc/containers/storage.conf": {Data: []byte("driver = \"overlay\"\n")},
		},
	})
	if err != nil {
		t.Fatalf("Overlay on a tree without drop-in dirs: %v", err)
	}
}
