package buildpodman

import (
	"archive/tar"
	"bufio"
	"context"
	"os"
	"path/filepath"
	"testing"

	seekable "github.com/SaveTheRbtz/zstd-seekable-format-go/pkg"
	"github.com/klauspost/compress/zstd"
)

// TestExtractInterpolateTransform exercises the real cross-package chain the
// installer runs (ExtractArtifact -> InterpolateTree -> TransformUserUnitsInDir)
// against a seekable-zstd artifact laid out as build produces it, including a
// generator symlink to confirm tarfs + os.CopyFS preserve it.
func TestExtractInterpolateTransform(t *testing.T) {
	src := t.TempDir()
	writeFile(t,
		filepath.Join(src, "etc/containers/storage.conf"),
		"graphroot = ${XDG_DATA_HOME}/g\n",
	)
	writeFile(t,
		filepath.Join(src, "etc/containers/path.env"),
		"PATH=${HOME}/.local/containers/bin\n",
	)
	writeFile(t,
		filepath.Join(src, "usr/local/lib/systemd/user/podman.service"),
		"[Service]\nExecStart=podman $LOGGING system service\n",
	)
	// A relative generator symlink, exactly as the distribution tree carries.
	writeFile(t, filepath.Join(src, "usr/local/libexec/podman/quadlet"), "binary\n")
	genDir := filepath.Join(src, "usr/local/lib/systemd/user-generators")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(
		"../../../libexec/podman/quadlet",
		filepath.Join(genDir, "podman-user-generator"),
	); err != nil {
		t.Fatal(err)
	}

	art := filepath.Join(t.TempDir(), "a.tar.zst")
	packSeekable(t, src, art)

	built := t.TempDir()
	if err := ExtractArtifact(art, built); err != nil {
		t.Fatal(err)
	}

	// The symlink must survive (tarfs HandleSymlink + os.CopyFS via ReadLinkFS).
	gen := filepath.Join(built, "usr/local/lib/systemd/user-generators/podman-user-generator")
	fi, err := os.Lstat(gen)
	if err != nil {
		t.Fatalf("generator symlink missing after extract: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("generator is a %v after extract, want symlink", fi.Mode())
	} else if tgt, _ := os.Readlink(gen); tgt != "../../../libexec/podman/quadlet" {
		t.Errorf("generator link target = %q", tgt)
	}

	env := InterpEnv{Home: "/home/u", XdgDataHome: "/home/u/.local/share"}
	if err := InterpolateTree(
		context.Background(),
		filepath.Join(built, "etc/containers"),
		env,
	); err != nil {
		t.Fatal(err)
	}
	if err := TransformUserUnitsInDir(
		filepath.Join(built, "usr/local/lib/systemd/user"),
		"/home/u/.config/containers/path.env",
		"/home/u/.local/containers/bin/podman",
	); err != nil {
		t.Fatal(err)
	}

	if got := readf(
		t,
		filepath.Join(built, "etc/containers/storage.conf"),
	); got != "graphroot = /home/u/.local/share/g\n" {
		t.Errorf("storage.conf not interpolated: %q", got)
	}
	unit := readf(t, filepath.Join(built, "usr/local/lib/systemd/user/podman.service"))
	want := "[Service]\nEnvironmentFile=/home/u/.config/containers/path.env\n" +
		"ExecStart=/home/u/.local/containers/bin/podman $LOGGING system service\n"
	if unit != want {
		t.Errorf("unit transform wrong:\ngot:  %q\nwant: %q", unit, want)
	}
}

// TestReadArtifactTag round-trips the root `tag` stamp through WriteArtifact and
// ReadArtifactTag, and confirms an unstamped archive reads back as "".
func TestReadArtifactTag(t *testing.T) {
	t.Run("stamped tag reads back trimmed", func(t *testing.T) {
		src := t.TempDir()
		writeFile(t, filepath.Join(src, "tag"), "v5.8.4\n")
		writeFile(t, filepath.Join(src, "etc/containers/storage.conf"), "graphroot = x\n")
		art := filepath.Join(t.TempDir(), "a.tar.zst")
		if err := WriteArtifact(src, art); err != nil {
			t.Fatal(err)
		}
		got, err := ReadArtifactTag(art)
		if err != nil {
			t.Fatal(err)
		}
		if got != "v5.8.4" {
			t.Errorf("ReadArtifactTag = %q, want %q", got, "v5.8.4")
		}
	})

	t.Run("absent tag reads back empty", func(t *testing.T) {
		src := t.TempDir()
		writeFile(t, filepath.Join(src, "etc/containers/storage.conf"), "graphroot = x\n")
		art := filepath.Join(t.TempDir(), "a.tar.zst")
		if err := WriteArtifact(src, art); err != nil {
			t.Fatal(err)
		}
		got, err := ReadArtifactTag(art)
		if err != nil {
			t.Fatal(err)
		}
		if got != "" {
			t.Errorf("ReadArtifactTag = %q, want empty for an unstamped archive", got)
		}
	})
}

// packSeekable writes srcDir as the seekable zstd tar that build produces
// (mirrors WriteArtifact for a self-contained fixture).
func packSeekable(t *testing.T, srcDir, outPath string) {
	t.Helper()
	out, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = out.Close() }()
	enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
	if err != nil {
		t.Fatal(err)
	}
	defer enc.Close()
	sw, err := seekable.NewWriter(out, enc)
	if err != nil {
		t.Fatal(err)
	}
	bw := bufio.NewWriterSize(sw, 1<<20)
	tw := tar.NewWriter(bw)
	if err := tw.AddFS(os.DirFS(srcDir)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := bw.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := sw.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
