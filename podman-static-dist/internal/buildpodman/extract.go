package buildpodman

import (
	"errors"
	"io/fs"
	"os"
	"strings"

	seekable "github.com/SaveTheRbtz/zstd-seekable-format-go/pkg"
	"github.com/klauspost/compress/zstd"
	"github.com/ngicks/go-fsys-helper/tarfs"
)

// ExtractArtifact expands the seekable zstd tar at tarPath into destDir. The
// seekable reader exposes the decompressed tar as an io.ReaderAt, tarfs presents
// that as an fs.FS (HandleSymlink keeps the generator symlinks), and os.CopyFS
// materializes it. destDir is removed first because os.CopyFS opens files with
// O_EXCL and refuses to overwrite an existing tree.
func ExtractArtifact(tarPath, destDir string) (err error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()

	dec, err := zstd.NewReader(nil)
	if err != nil {
		return err
	}
	defer dec.Close()

	sr, err := seekable.NewReader(f, dec)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := sr.Close(); err == nil {
			err = cerr
		}
	}()

	fsys, err := tarfs.New(sr, &tarfs.FsOption{HandleSymlink: true})
	if err != nil {
		return err
	}
	if err := os.RemoveAll(destDir); err != nil {
		return err
	}
	return os.CopyFS(destDir, fsys)
}

// ReadArtifactTag returns the trimmed contents of the root `tag` file stamped
// into the artifact at tarPath (build writes it next to etc/ and usr/). It reuses
// the same seekable-zstd reader + tarfs plumbing as ExtractArtifact. Archives
// produced before the stamp existed have no such file; ReadArtifactTag returns
// ("", nil) for them.
func ReadArtifactTag(tarPath string) (tag string, err error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return "", err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()

	dec, err := zstd.NewReader(nil)
	if err != nil {
		return "", err
	}
	defer dec.Close()

	sr, err := seekable.NewReader(f, dec)
	if err != nil {
		return "", err
	}
	defer func() {
		if cerr := sr.Close(); err == nil {
			err = cerr
		}
	}()

	fsys, err := tarfs.New(sr, &tarfs.FsOption{HandleSymlink: true})
	if err != nil {
		return "", err
	}
	b, err := fs.ReadFile(fsys, "tag")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
