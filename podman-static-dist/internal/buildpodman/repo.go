package buildpodman

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Sync ensures dir holds url checked out at tag. An existing work tree is
// fetched and re-checked-out; otherwise url is cloned fresh. git's output is
// forwarded to stderr so progress and errors reach the user.
func Sync(ctx context.Context, dir, url, tag string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found on PATH (required to fetch %s): %w", url, err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		if err := git(ctx, dir, "fetch", "--tags", "--force", "origin"); err != nil {
			return fmt.Errorf("fetching %s: %w", url, err)
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return err
		}
		if err := git(ctx, "", "clone", url, dir); err != nil {
			return fmt.Errorf("cloning %s: %w", url, err)
		}
	}
	if err := git(ctx, dir, "checkout", "-f", "tags/"+tag); err != nil {
		return fmt.Errorf("checking out tag %s: %w", tag, err)
	}
	return nil
}

// git runs one git command in dir (empty dir means the process working
// directory). stdout is redirected to stderr to keep the tool's own stdout
// clean while still surfacing git's progress.
func git(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
