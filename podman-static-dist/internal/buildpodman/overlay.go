package buildpodman

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// OverlayParams configures Overlay.
type OverlayParams struct {
	AssetDir   string // the distribution tree `make singlearch-tar` produced
	ResourceFS fs.FS  // our resource tree; copied over AssetDir
}

// prunedDirs are upstream drop-in directories removed from the assembled tree
// before our resources are overlaid, relative to AssetDir.
//
// Podman 6 reads `<conf>.d/` next to the user config, and drop-ins win over the
// main file. Upstream ships etc/containers/storage.conf.d/00-podman-static.conf
// with mount_program hardcoded to /usr/local/bin/fuse-overlayfs, which does not
// exist on a host using this dist and so overrides the dist-relative path our
// storage.conf sets. Our whole-file configs already carry every setting the
// drop-ins hold, so dropping the directories is safe. containers.conf.d is
// pruned for the same reason should upstream start shipping one.
var prunedDirs = []string{
	"etc/containers/storage.conf.d",
	"etc/containers/containers.conf.d",
}

// Overlay writes our resource tree on top of the tree that upstream's
// `make singlearch-tar` already assembled: upstream drop-in directories
// (prunedDirs) are removed, then every file in ResourceFS is copied over the
// matching AssetDir path, creating directories and overwriting files verbatim.
// Interpolation is deferred to install time.
func Overlay(ctx context.Context, p OverlayParams) error {
	for _, dir := range prunedDirs {
		if err := os.RemoveAll(filepath.Join(p.AssetDir, filepath.FromSlash(dir))); err != nil {
			return fmt.Errorf("pruning %s: %w", dir, err)
		}
	}
	return copyTree(ctx, p.ResourceFS, p.AssetDir)
}

// copyTree copies every regular file in srcFS into dst, recreating the source
// directory structure and overwriting existing files. Files are written 0644
// (embed.FS reports its files read-only, so the source mode is not kept).
func copyTree(ctx context.Context, srcFS fs.FS, dst string) error {
	return fs.WalkDir(srcFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		target := filepath.Join(dst, filepath.FromSlash(path))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("resource tree contains non-regular file: %s", path)
		}
		data, err := fs.ReadFile(srcFS, path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
