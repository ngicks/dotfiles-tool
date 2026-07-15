// Package build produces the distribution artifact: it provisions a Lima VM,
// builds podman-static inside it with docker, then assembles the tree and
// compresses it to a tar.zst on the host.
//
// The heavy build runs in the VM (one `docker build --output=type=local`); the
// host reads the exported filesystem back over the shared mount and does the
// layout + compression in Go (see internal/buildpodman).
package build

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ngicks/dotfiles-tool/podman-static-dist/internal/buildpodman"
	"github.com/ngicks/dotfiles-tool/podman-static-dist/internal/lima"
)

const repoUrl = "https://github.com/mgoltzsche/podman-static"

// Option configures Run.
type Option struct {
	Tag      string // required: podman-static tag to build (e.g. v5.8.4)
	Resource fs.FS  // required: resource tree; its directories are copied over the built tree
	// OutputPath is the destination .tar.zst; empty defaults to
	// <standard base>/out/podman-static-<tag>.tar.zst.
	OutputPath string
	Recreate   bool        // recreate the VM before building
	Vm         lima.Config // VM config; HostWork defaults to the standard base dir
	// Confirm, when set, is asked before provisioning a fresh (slow) VM. A
	// false return aborts the build.
	Confirm func(prompt string) (bool, error)
}

// Defaults returns an Option seeded with the default VM configuration. Callers
// bind flags onto it and set the required fields (Tag, ResourceFS, OutputPath)
// before calling Run.
func Defaults() Option {
	return Option{Vm: lima.Defaults()}
}

// Validate reports whether the required fields are set. Run calls it before use.
func (o Option) Validate() error {
	if o.Tag == "" {
		return fmt.Errorf("tag is required")
	}
	if o.Resource == nil {
		return fmt.Errorf("resource fs is required")
	}
	if o.Vm.Name == "" {
		return fmt.Errorf("vm name is required")
	}
	return nil
}

// Run performs the full build.
func Run(ctx context.Context, o Option) error {
	if err := o.Validate(); err != nil {
		return err
	}

	if o.OutputPath == "" || o.Vm.HostWork == "" {
		base, err := standardBaseDir()
		if err != nil {
			return err
		}
		if o.OutputPath == "" {
			o.OutputPath = defaultOutputPath(base, o.Tag)
		}
		if o.Vm.HostWork == "" {
			o.Vm.HostWork = base
		}
	}

	cli, err := lima.FindCliFromPath()
	if err != nil {
		return err
	}

	vm := o.Vm
	vm.HostWork, err = filepath.Abs(vm.HostWork)
	if err != nil {
		return fmt.Errorf("resolving work dir: %w", err)
	}

	status, err := cli.Status(ctx, vm.Name)
	if err != nil {
		return err
	}

	// A persistent instance keeps the mount it was created with. If this build's
	// work dir differs from the reused instance's, reusing it would build against
	// a stale directory (Lima does not re-apply mounts on reuse), so recreate —
	// alongside an explicit -recreate.
	recreate := o.Recreate
	if status != "" && !recreate {
		mount, err := cli.MountLocation(ctx, vm.Name)
		if err != nil {
			return err
		}
		if mount != "" && !sameDir(mount, vm.HostWork) {
			recreate = true
		}
	}

	// Confirm before a fresh create/recreate: it is slow.
	if o.Confirm != nil && (status == "" || recreate) {
		ok, err := o.Confirm(fmt.Sprintf("Lima VM %q must be %s (this is slow). Proceed?",
			vm.Name, ternary(status == "", "created", "recreated")))
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("aborted by user")
		}
	}

	if _, err := cli.Ensure(ctx, vm, recreate); err != nil {
		return fmt.Errorf("provisioning VM: %w", err)
	}

	// Check out podman-static on the host (git lives here, not in the VM). The
	// work tree sits on the shared mount, so the in-VM docker build reads it.
	repoDir := filepath.Join(vm.HostWork, "podman-static")
	if err := buildpodman.Sync(ctx, repoDir, repoUrl, o.Tag); err != nil {
		return fmt.Errorf("checking out podman-static: %w", err)
	}

	// The Lima docker template runs rootless docker, so no sudo is needed.
	if err := cli.RunScript(
		ctx,
		vm.Name,
		buildScript(lima.GuestPath("podman-static")),
	); err != nil {
		return fmt.Errorf("building in VM: %w", err)
	}

	// `make singlearch-tar` assembled the full distribution tree under the repo's
	// build dir (on the shared mount); we only overlay our config on top of it.
	assetDir := filepath.Join(repoDir, "build/asset/podman-linux-amd64")
	if err := buildpodman.Overlay(ctx, buildpodman.OverlayParams{
		AssetDir:   assetDir,
		ResourceFS: o.Resource,
	}); err != nil {
		return fmt.Errorf("overlaying config: %w", err)
	}

	// Stamp the built tag into the tree root so it rides into the archive next to
	// etc/ and usr/; install/extract read it back to make --tag optional.
	if err := os.WriteFile(filepath.Join(assetDir, "tag"), []byte(o.Tag+"\n"), 0o644); err != nil {
		return fmt.Errorf("stamping tag: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(o.OutputPath), 0o755); err != nil {
		return err
	}
	if err := buildpodman.WriteArtifact(assetDir, o.OutputPath); err != nil {
		return fmt.Errorf("writing artifact: %w", err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", o.OutputPath)
	return nil
}

// buildScript is the bash run inside the VM: it delegates the whole distribution
// layout to upstream's Makefile — `make singlearch-tar` builds the tar-archive
// image and assembles build/asset/podman-linux-<arch>, and `make create-builder`
// first creates the buildx builder the Makefile targets by name.
//
// make is installed on demand: Lima's docker template does not ship it, and
// provisioning it at boot races the template's own apt install for the dpkg
// lock, so we install it here once the VM is idle (DPkg::Lock::Timeout waits out
// any straggler). Only that one-time install uses sudo (passwordless in the
// docker template); the build itself runs rootless. The repo is checked out on
// the host (see Sync), so the VM needs docker + make but not git.
func buildScript(repoPath string) string {
	return fmt.Sprintf(`
set -eu
command -v make >/dev/null 2>&1 || {
  sudo apt-get -o DPkg::Lock::Timeout=300 update
  sudo DEBIAN_FRONTEND=noninteractive apt-get -o DPkg::Lock::Timeout=300 install -y make
}
cd %q
make create-builder
make singlearch-tar PLATFORM=linux/amd64
`, repoPath)
}

// standardBaseDir is the standard host location for build state:
// <user cache dir>/dotfiles/build/podman-static. The --work default is the dir
// itself; the -o default lives under out/ beneath it.
func standardBaseDir() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolving user cache dir: %w", err)
	}
	return filepath.Join(cache, "dotfiles", "build", "podman-static"), nil
}

// defaultOutputPath places the artifact under the standard base dir, named
// after the built tag so the devenv image build can locate it.
func defaultOutputPath(base, tag string) string {
	return filepath.Join(base, "out", "podman-static-"+tag+".tar.zst")
}

// sameDir reports whether a and b resolve to the same directory.
func sameDir(a, b string) bool {
	aa, erra := filepath.Abs(a)
	bb, errb := filepath.Abs(b)
	if erra != nil || errb != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	if r, err := filepath.EvalSymlinks(aa); err == nil {
		aa = r
	}
	if r, err := filepath.EvalSymlinks(bb); err == nil {
		bb = r
	}
	return aa == bb
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
