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

// Overlay writes our resource tree on top of the tree that upstream's
// `make singlearch-tar` already assembled: every file in ResourceFS is copied
// over the matching AssetDir path, creating directories and overwriting files
// verbatim. Interpolation is deferred to install time.
func Overlay(ctx context.Context, p OverlayParams) error {
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
