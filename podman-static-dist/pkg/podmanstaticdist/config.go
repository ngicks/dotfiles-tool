package podmanstaticdist

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/caarlos0/env/v11"

	"github.com/ngicks/dotfiles-tool/podman-static-dist/internal/lima"
	"github.com/ngicks/dotfiles-tool/podman-static-dist/rc"
)

type Config struct {
	Tag         string `json:"tag" yaml:"tag"`
	VMName      string `json:"vm_name" yaml:"vm_name"`
	ArtifactDir string `json:"artifact_dir" yaml:"artifact_dir"`
}

func DefaultConfig() Config {
	return Config{
		Tag:         rc.Tag(),
		VMName:      lima.Defaults().Name,
		ArtifactDir: "",
	}
}

//nolint:lll // triple json/yaml/env tags; one field per line, never wrap tags
type PartialConfig struct {
	Tag         *string `json:"tag,omitzero" yaml:"tag,omitempty" env:"TAG"`
	VMName      *string `json:"vm_name,omitzero" yaml:"vm_name,omitempty" env:"VM_NAME"`
	ArtifactDir *string `json:"artifact_dir,omitzero" yaml:"artifact_dir,omitempty" env:"ARTIFACT_DIR"`
}

func (p PartialConfig) Apply(base Config) Config {
	if p.Tag != nil {
		base.Tag = *p.Tag
	}
	if p.VMName != nil {
		base.VMName = *p.VMName
	}
	if p.ArtifactDir != nil {
		base.ArtifactDir = *p.ArtifactDir
	}
	return base
}

var envOptions = env.Options{Prefix: "PODMAN_STATIC_DIST_"}

func LoadConfig(flagPath string) (Config, error) {
	cfg := DefaultConfig()

	path, err := configPath(flagPath)
	if err != nil {
		return cfg, err
	}
	filePartial, err := unmarshalConfigFile(path)
	if err != nil {
		return cfg, err
	}
	cfg = filePartial.Apply(cfg)

	var envPartial PartialConfig
	if err := env.ParseWithOptions(&envPartial, envOptions); err != nil {
		return cfg, err
	}
	cfg = envPartial.Apply(cfg)

	return cfg, nil
}

func unmarshalConfigFile(path string) (PartialConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return PartialConfig{}, nil
		}
		return PartialConfig{}, fmt.Errorf("read config %q: %w", path, err)
	}
	var p PartialConfig
	if err := json.Unmarshal(b, &p); err != nil {
		return PartialConfig{}, fmt.Errorf("parse config %q: %w", path, err)
	}
	return p, nil
}

const envConfVar = "PODMAN_STATIC_DIST_CONF"

func configPath(flagPath string) (string, error) {
	if flagPath != "" {
		return flagPath, nil
	}
	if p, ok := os.LookupEnv(envConfVar); ok {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "devenv", "build", "podman-static", "config.json"), nil
}
