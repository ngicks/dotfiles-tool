// Package install extracts a podman-static dist tarball and wires it into the caller's home.
package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ngicks/dotfiles-tool/podman-static-dist/internal/buildpodman"
)

type Env struct {
	Home        string
	DataHome    string
	ConfigHome  string
	ArtifactDir string
}

func ResolveEnvFromOS() (Env, error) {
	return ResolveEnv(os.LookupEnv)
}

func ResolveEnv(lookup func(string) (string, bool)) (Env, error) {
	home, ok := lookup("HOME")
	if !ok || home == "" {
		return Env{}, fmt.Errorf("environment variable HOME is not set")
	}
	get := func(key, def string) string {
		if v, ok := lookup(key); ok && v != "" {
			return v
		}
		return def
	}
	return Env{
		Home:        home,
		DataHome:    get("XDG_DATA_HOME", filepath.Join(home, ".local/share")),
		ConfigHome:  get("XDG_CONFIG_HOME", filepath.Join(home, ".config")),
		ArtifactDir: get("TARGET_ARTIFACT_DIR", ""),
	}, nil
}

func (e Env) podmanBase() string {
	if e.ArtifactDir != "" {
		return e.ArtifactDir
	}
	return filepath.Join(e.DataHome, "podman-dist")
}

type Option struct {
	TarPath string
	// Tag is the explicit install tag (from --tag). When empty, the tag is
	// resolved from the archive stamp, then TagFallback.
	Tag string
	// TagFallback is the config/embedded default, used only when neither an
	// explicit Tag nor the archive's stamped tag supplies one.
	TagFallback string
	Env         Env
}

func Defaults() Option {
	return Option{}
}

func (o Option) Validate() error {
	if o.TarPath == "" {
		return fmt.Errorf("tar path is required")
	}
	if o.Env.Home == "" {
		return fmt.Errorf("env HOME is required")
	}
	return nil
}

// resolveTag applies the precedence explicit --tag > archive stamp >
// config/embedded default, erroring only when all three are empty.
func (o Option) resolveTag() (string, error) {
	if o.Tag != "" {
		return o.Tag, nil
	}
	archiveTag, err := buildpodman.ReadArtifactTag(o.TarPath)
	if err != nil {
		return "", fmt.Errorf("reading archive tag: %w", err)
	}
	if archiveTag != "" {
		return archiveTag, nil
	}
	if o.TagFallback != "" {
		return o.TagFallback, nil
	}
	return "", fmt.Errorf("tag is required: pass --tag, use a stamped archive, or set config tag")
}

// Run extracts the artifact, then links it. An empty l.Base or l.Tag defaults
// to the env base and the resolved tag; l.Env is always taken from o.Env, so
// both stages agree on where the tree lives.
func Run(ctx context.Context, o Option, l LinkOption) error {
	if err := o.Validate(); err != nil {
		return err
	}
	// Resolve once so Extract and Link agree; setting o.Tag short-circuits the
	// resolveTag Extract runs (no second archive read).
	tag, err := o.resolveTag()
	if err != nil {
		return err
	}
	o.Tag = tag
	if err := Extract(ctx, o); err != nil {
		return err
	}
	if l.Base == "" {
		l.Base = o.Env.podmanBase()
	}
	if l.Tag == "" {
		l.Tag = tag
	}
	l.Env = o.Env
	return Link(ctx, l)
}

func Extract(ctx context.Context, o Option) error {
	if err := o.Validate(); err != nil {
		return err
	}
	tag, err := o.resolveTag()
	if err != nil {
		return err
	}
	base := o.Env.podmanBase()
	destDir := filepath.Join(base, tag)
	tmpDir := destDir + ".tmp"

	// Extract, interpolate, and transform against a tmp dir, then publish it with
	// a single rename so destDir never holds a partially-written tree. The defer
	// clears the tmp dir on any failure; after a successful rename it is gone, so
	// the RemoveAll is a no-op.
	if err := os.RemoveAll(tmpDir); err != nil {
		return fmt.Errorf("removing stale tmp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := buildpodman.ExtractArtifact(o.TarPath, tmpDir); err != nil {
		return fmt.Errorf("extracting %s: %w", o.TarPath, err)
	}

	ienv := buildpodman.InterpEnv{Home: o.Env.Home, XdgDataHome: o.Env.DataHome}
	if err := buildpodman.InterpolateTree(
		ctx,
		filepath.Join(tmpDir, "etc/containers"),
		ienv,
	); err != nil {
		return fmt.Errorf("interpolating conf: %w", err)
	}
	if err := buildpodman.InterpolateTree(
		ctx,
		filepath.Join(tmpDir, "etc/environment.d"),
		ienv,
	); err != nil {
		return fmt.Errorf("interpolating environment.d: %w", err)
	}

	envFile := filepath.Join(o.Env.ConfigHome, "containers/path.env")
	podmanPath := filepath.Join(base, "current/usr/local/bin/podman")
	if err := buildpodman.TransformUserUnitsInDir(
		filepath.Join(tmpDir, "usr/local/lib/systemd/user"),
		envFile,
		podmanPath,
	); err != nil {
		return fmt.Errorf("transforming user units: %w", err)
	}

	if err := os.RemoveAll(destDir); err != nil {
		return fmt.Errorf("removing prior dist dir: %w", err)
	}
	if err := os.Rename(tmpDir, destDir); err != nil {
		return fmt.Errorf("publishing dist dir: %w", err)
	}
	return nil
}
