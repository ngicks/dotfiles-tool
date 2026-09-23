package install

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// updateSymlink points linkPath at target, replacing an existing symlink but
// refusing to touch a real file or directory.
func updateSymlink(target, linkPath string) error {
	if fi, err := os.Lstat(linkPath); err == nil {
		if fi.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("refusing to replace non-symlink: %s", linkPath)
		}
		if cur, err := os.Readlink(linkPath); err == nil && cur == target {
			return nil
		}
		if err := os.Remove(linkPath); err != nil {
			return err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		return err
	}
	return os.Symlink(target, linkPath)
}

func forceSymlink(target, linkPath string) error {
	if fi, err := os.Lstat(linkPath); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		if cur, err := os.Readlink(linkPath); err == nil && cur == target {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		return err
	}
	if err := os.Remove(linkPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.Symlink(target, linkPath)
}

func needElevate(dir string) bool {
	if os.Geteuid() == 0 {
		return false
	}
	fi, err := os.Stat(dir)
	if err != nil {
		return true
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return true
	}
	mode := fi.Mode().Perm()
	switch {
	case int(st.Uid) == os.Geteuid():
		return mode&0o200 == 0
	case int(st.Gid) == os.Getegid():
		return mode&0o020 == 0
	default:
		return mode&0o002 == 0
	}
}

func elevate(ctx context.Context, name string, args ...string) error {
	argv := append([]string{name}, args...)
	if os.Geteuid() != 0 {
		argv = append([]string{"sudo"}, argv...)
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
