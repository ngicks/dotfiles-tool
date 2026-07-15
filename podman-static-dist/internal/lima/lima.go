// Package lima drives a Lima VM used to build podman-static reproducibly,
// independent of the host runtime (macOS or Linux).
//
// The `build` command always builds inside the VM (an intentional design
// choice): the VM is provisioned from Lima's docker template — rootless docker,
// buildx included — and a host directory is mounted writable so the exported
// image filesystem and checked-out repo are read back on the host without any
// extra copy. A persistent, named instance is reused across builds; -recreate
// tears it down first.
//
// It requires Lima 2.0+ (for the `template:` locator form used in `base:`
// inheritance) and shells out only to `limactl`, via LimactlCli.
package lima

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"

	"go.yaml.in/yaml/v4"
)

// dnsProvisionScript fixes DNS resolution for the in-VM rootless docker; see
// dns-provision.sh for why it is needed.
//
//go:embed dns-provision.sh
var dnsProvisionScript string

// The build VM's shape is fixed, not user-configurable: the guest is fully
// isolated and always runs the same build, so exposing these as knobs only
// invited confusion (a persistent instance keeps whatever size it was created
// with, silently ignoring a changed flag on reuse).
//
//   - baseTemplate: Lima's docker template (rootless docker + buildx).
//   - mountPoint:   where HostWork is mounted inside the VM.
//   - diskSize:     ample and stable for this build's fixed workload.
//
// vCPUs match the host (runtime.NumCPU) and memory is sized per host; see
// instanceYaml and vmMemory (hostmem.go).
const (
	baseTemplate = "template:docker"
	mountPoint   = "/mnt/psbuild"
	diskSize     = "60GiB"
)

// GuestPath returns where a path relative to HostWork appears inside the VM.
func GuestPath(rel string) string {
	return path.Join(mountPoint, rel)
}

// Config describes the build VM. Only the instance identity and the host work
// dir are configurable; the VM's sizing and base image are fixed internals (see
// the const block above).
type Config struct {
	Name     string // instance name (persistent, reused across builds)
	HostWork string // host directory mounted into the VM (writable)
}

// Defaults returns a Config with the default instance name; HostWork must be set
// by the caller (it is where build artifacts are exchanged).
func Defaults() Config {
	return Config{
		Name: "podman-static-build",
	}
}

// LimactlCli is a resolved `limactl` binary used to drive Lima instances. It
// stores only the executable path; construct it with FindCliFromPath. Its
// methods forward that path to the package's unexported implementations.
type LimactlCli struct {
	path string
}

// FindCliFromPath resolves the limactl binary on PATH, returning actionable
// guidance when Lima is not installed.
func FindCliFromPath() (*LimactlCli, error) {
	path, err := exec.LookPath("limactl")
	if err != nil {
		return nil, fmt.Errorf("limactl not found: install Lima (https://lima-vm.io), " +
			"e.g. `brew install lima`, `nix profile install nixpkgs#lima`, or `mise use -g lima`")
	}
	return &LimactlCli{path: path}, nil
}

// Status returns the instance status ("Running", "Stopped", ...) or "" if the
// instance does not exist.
func (l *LimactlCli) Status(ctx context.Context, name string) (string, error) {
	return status(ctx, l.path, name)
}

// MountLocation returns the host directory the named instance currently mounts
// as its work dir, or "" if the instance does not exist or has no such mount.
// A persistent instance keeps whatever mount it was created with, so the caller
// can compare this against the work dir it wants to detect a stale reuse.
func (l *LimactlCli) MountLocation(ctx context.Context, name string) (string, error) {
	return mountLocation(ctx, l.path, name)
}

// Ensure makes the named instance exist and run. When recreate is true an
// existing instance is deleted first. Returns whether a fresh instance was
// created (the caller may want to warn the user about the slow first build).
func (l *LimactlCli) Ensure(
	ctx context.Context,
	c Config,
	recreate bool,
) (created bool, err error) {
	return ensure(ctx, l.path, c, recreate)
}

// RunScript runs a bash script inside the instance, forwarding stdio so build
// output (and any prompt) reaches the user.
func (l *LimactlCli) RunScript(ctx context.Context, name, script string) error {
	return runScript(ctx, l.path, name, script)
}

// instanceRecord is the subset of `limactl list --json` this tool reads.
type instanceRecord struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Dir    string `json:"dir"` // instance dir holding the rendered lima.yaml
}

// lookup returns the record for name, or ok=false if no such instance exists.
func lookup(ctx context.Context, limactl, name string) (instanceRecord, bool, error) {
	out, err := exec.CommandContext(ctx, limactl, "list", "--json").Output()
	if err != nil {
		return instanceRecord{}, false, fmt.Errorf("limactl list: %w", err)
	}
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var inst instanceRecord
		if err := json.Unmarshal(line, &inst); err != nil {
			return instanceRecord{}, false, fmt.Errorf("parsing limactl list output: %w", err)
		}
		if inst.Name == name {
			return inst, true, nil
		}
	}
	return instanceRecord{}, false, sc.Err()
}

func status(ctx context.Context, limactl, name string) (string, error) {
	rec, ok, err := lookup(ctx, limactl, name)
	if err != nil || !ok {
		return "", err
	}
	return rec.Status, nil
}

// mountLocation returns the host dir the instance mounts at mountPoint, or "" if
// the instance is absent or has no such mount. `limactl list --json` omits
// mounts, so it is read from the instance's rendered lima.yaml.
func mountLocation(ctx context.Context, limactl, name string) (string, error) {
	rec, ok, err := lookup(ctx, limactl, name)
	if err != nil || !ok {
		return "", err
	}
	b, err := os.ReadFile(filepath.Join(rec.Dir, "lima.yaml"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	var cfg struct {
		Mounts []struct {
			Location   string `yaml:"location"`
			MountPoint string `yaml:"mountPoint"`
		} `yaml:"mounts"`
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return "", fmt.Errorf("parsing instance lima.yaml: %w", err)
	}
	for _, m := range cfg.Mounts {
		if m.MountPoint == mountPoint {
			return m.Location, nil
		}
	}
	return "", nil
}

func ensure(
	ctx context.Context,
	limactl string,
	c Config,
	recreate bool,
) (created bool, err error) {
	st, err := status(ctx, limactl, c.Name)
	if err != nil {
		return false, err
	}
	if recreate && st != "" {
		if err := run(ctx, limactl, "delete", "--force", c.Name); err != nil {
			return false, err
		}
		st = ""
	}
	switch st {
	case "":
		return true, create(ctx, limactl, c)
	case "Running":
		return false, nil
	default:
		return false, run(ctx, limactl, "start", "--tty=false", c.Name)
	}
}

// create writes an instance config inheriting the docker template and starts it.
func create(ctx context.Context, limactl string, c Config) error {
	if err := os.MkdirAll(c.HostWork, 0o755); err != nil {
		return fmt.Errorf("creating host work dir: %w", err)
	}
	body, err := c.instanceYaml()
	if err != nil {
		return fmt.Errorf("rendering instance config: %w", err)
	}
	f, err := os.CreateTemp("", "lima-podman-static-*.yaml")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err := f.Write(body); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return run(ctx, limactl, "start", "--tty=false", "--name="+c.Name, f.Name())
}

func runScript(ctx context.Context, limactl, name, script string) error {
	return run(ctx, limactl, "shell", name, "--", "bash", "-euo", "pipefail", "-c", script)
}

func run(ctx context.Context, limactl string, args ...string) error {
	cmd := exec.CommandContext(ctx, limactl, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// instanceConfig is the subset of Lima's instance YAML this tool emits; the rest
// is inherited from the base template referenced by Base.
type instanceConfig struct {
	Base      string          `yaml:"base"`
	Cpus      int             `yaml:"cpus"`
	Memory    string          `yaml:"memory"`
	Disk      string          `yaml:"disk"`
	Mounts    []instanceMount `yaml:"mounts"`
	Provision []provisionStep `yaml:"provision"`
}

type instanceMount struct {
	Location   string `yaml:"location"`
	MountPoint string `yaml:"mountPoint"`
	Writable   bool   `yaml:"writable"`
}

type provisionStep struct {
	Mode   string `yaml:"mode"`
	Script string `yaml:"script"`
}

// instanceYaml marshals the Lima instance config. It always includes the DNS
// provision (dnsProvisionScript) because rootless docker's gvisor-tap-vsock
// network namespace cannot reach systemd-resolved's 127.0.0.53 stub.
func (c Config) instanceYaml() ([]byte, error) {
	memory, err := vmMemory()
	if err != nil {
		return nil, err
	}
	return yaml.Marshal(instanceConfig{
		Base:   baseTemplate,
		Cpus:   runtime.NumCPU(),
		Memory: memory,
		Disk:   diskSize,
		Mounts: []instanceMount{{
			Location:   c.HostWork,
			MountPoint: mountPoint,
			Writable:   true,
		}},
		Provision: []provisionStep{{
			Mode:   "system",
			Script: dnsProvisionScript,
		}},
	})
}
