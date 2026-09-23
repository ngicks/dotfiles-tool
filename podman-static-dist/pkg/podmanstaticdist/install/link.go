package install

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"iter"
	"os"
	"os/exec"
	"path/filepath"
)

type LinkOption struct {
	Base        string
	Tag         string
	SkipSystemd bool
	Env         Env
}

func (o LinkOption) Validate() error {
	if o.Env.Home == "" {
		return fmt.Errorf("env HOME is required")
	}
	return nil
}

func Link(ctx context.Context, o LinkOption) error {
	if err := o.Validate(); err != nil {
		return err
	}

	base := o.Base
	if base == "" {
		base = o.Env.podmanBase()
	}
	current := filepath.Join(base, "current")

	if o.Tag != "" {
		if err := updateSymlink(o.Tag, current); err != nil {
			fi, statErr := os.Lstat(current)
			if statErr != nil || fi.Mode()&os.ModeSymlink == 0 {
				return fmt.Errorf("linking current -> %s: %w", o.Tag, err)
			}
			fmt.Fprintf(os.Stderr,
				"notice: base %q is not writable; keeping existing current symlink\n", base)
		}
	}
	if _, err := os.Lstat(current); err != nil {
		return fmt.Errorf(
			"no current symlink at %s (pass --tag on a writable base to create it): %w",
			current, err)
	}

	systemd := !o.SkipSystemd
	if systemd {
		if _, err := exec.LookPath("systemctl"); err != nil {
			fmt.Fprintln(os.Stderr, "notice: systemctl not found on PATH; skipping "+
				"systemd unit links, quadlet generator, and daemon-reload")
			systemd = false
		}
	}

	rules, err := wiringRules(wiringParams{
		env:     o.Env,
		current: current,
		systemd: systemd,
	})
	if err != nil {
		return fmt.Errorf("building link table: %w", err)
	}

	if err := applyLinks(rules); err != nil {
		return err
	}

	if systemd {
		if err := installQuadletGenerator(ctx, current); err != nil {
			return err
		}
		daemonReload(ctx)
	}
	return nil
}

type linkRule = [2]string

type wiringParams struct {
	env     Env    // base home/config paths used to build link sources and targets
	current string // current dir; enumeration root and link target base
	systemd bool   // whether to emit systemd/user unit link rows
}

func wiringRules(p wiringParams) ([]linkRule, error) {
	configDir := filepath.Join(p.env.ConfigHome, "containers")

	links := []linkRule{
		// ~/.config/containers -> ~/.local/share/podman-dist/current/etc/containers
		{configDir, filepath.Join(p.current, "etc/containers")},
	}

	for name, err := range listDirent(
		filepath.Join(p.current, "etc/environment.d"),
		func(e fs.DirEntry) bool { return e.Type().IsRegular() },
	) {
		if err != nil {
			return nil, err
		}
		// ~/.config/environment.d/* -> ~/.local/share/podman-dist/current/etc/environment.d/*
		links = append(links, linkRule{
			filepath.Join(p.env.ConfigHome, "environment.d", name),
			filepath.Join(p.current, "etc/environment.d", name),
		})
	}

	for name, err := range listDirent(
		filepath.Join(p.current, "usr/local/share/bash-completion/completions"),
		func(e fs.DirEntry) bool { return e.Type().IsRegular() },
	) {
		if err != nil {
			return nil, err
		}
		// ~/.local/share/bash-completion/completions/* ->
		// ~/.local/share/podman-dist/current/usr/local/share/bash-completion/completions/*
		//
		// bash-completion lazy-loads per-command files from this XDG user dir, so a
		// link is all bash needs. zsh has no such user dir; its fpath entry points
		// straight at the dist tree from etc/containers/path.sh instead.
		links = append(links, linkRule{
			filepath.Join(p.env.DataHome, "bash-completion/completions", name),
			filepath.Join(p.current, "usr/local/share/bash-completion/completions", name),
		})
	}

	if p.systemd {
		for name, err := range listDirent(
			filepath.Join(p.current, "usr/local/lib/systemd/user"),
			func(e fs.DirEntry) bool { return e.Type().IsRegular() },
		) {
			if err != nil {
				return nil, err
			}
			// ~/.config/systemd/user/* ->
			// ~/.local/share/podman-dist/current/usr/local/lib/systemd/user/*
			links = append(links, linkRule{
				filepath.Join(p.env.ConfigHome, "systemd/user", name),
				filepath.Join(p.current, "usr/local/lib/systemd/user", name),
			})
		}
	}

	return links, nil
}

func listDirent(dir string, filter func(fs.DirEntry) bool) iter.Seq2[string, error] {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return func(yield func(string, error) bool) {}
		}
		return func(yield func(string, error) bool) { yield("", err) }
	}
	return func(yield func(string, error) bool) {
		for _, e := range entries {
			if filter(e) && !yield(e.Name(), nil) {
				return
			}
		}
	}
}

func applyLinks(rules []linkRule) error {
	for _, r := range rules {
		if err := updateSymlink(r[1], r[0]); err != nil {
			return fmt.Errorf("linking %s: %w", r[0], err)
		}
	}
	return nil
}

func installQuadletGenerator(ctx context.Context, current string) error {
	quadlet := filepath.Join(current, "usr/local/libexec/podman/quadlet")
	if info, err := os.Stat(quadlet); err != nil || info.Mode()&0o111 == 0 {
		fmt.Fprintf(os.Stderr, "warning: quadlet binary not found or not executable: %s\n", quadlet)
		return nil
	}
	const genName = "podman-user-generator"
	primary := "/usr/local/lib/systemd/user-generators"
	fallback := "/run/systemd/user-generators"

	var lastErr error
	for _, dir := range []string{primary, fallback} {
		link := filepath.Join(dir, genName)
		alreadyLinked := false
		if fi, err := os.Lstat(link); err == nil && fi.Mode()&os.ModeSymlink != 0 {
			if cur, err := os.Readlink(link); err == nil && cur == quadlet {
				alreadyLinked = true
			}
		}
		if !alreadyLinked {
			if !needElevate(dir) {
				if err := forceSymlink(quadlet, link); err != nil {
					lastErr = err
					continue
				}
			} else {
				if _, err := exec.LookPath("sudo"); err != nil {
					fmt.Fprintf(
						os.Stderr,
						"warning: skipping quadlet generator (need root or sudo); to enable it, run as root:\n"+
							"  ln -sfn %s %s\n",
						quadlet,
						filepath.Join(primary, genName),
					)
					return nil
				}
				if err := elevate(ctx, "mkdir", "-p", dir); err != nil {
					lastErr = err
					continue
				}
				if err := elevate(ctx, "ln", "-sfn", quadlet, link); err != nil {
					lastErr = err
					continue
				}
			}
		}
		fmt.Printf("installed quadlet user generator: %s\n", link)
		if dir == fallback {
			fmt.Fprintln(
				os.Stderr,
				"warning: /run is tmpfs; rerun this installer after reboot if the generator disappears",
			)
		}
		return nil
	}
	return fmt.Errorf("installing quadlet generator: %w", lastErr)
}

func daemonReload(ctx context.Context) {
	cmd := exec.CommandContext(ctx, "systemctl", "--user", "daemon-reload")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: systemctl --user daemon-reload failed: %v\n", err)
	}
}
