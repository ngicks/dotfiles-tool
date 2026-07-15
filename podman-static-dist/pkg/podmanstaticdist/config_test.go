package podmanstaticdist

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPartialConfigApply(t *testing.T) {
	base := Config{
		Tag:         "base-tag",
		VMName:      "base-vm",
		ArtifactDir: "base-dir",
	}

	t.Run("nil fields leave base untouched", func(t *testing.T) {
		got := PartialConfig{}.Apply(base)
		if !reflect.DeepEqual(got, base) {
			t.Errorf("empty partial changed base:\ngot  %+v\nwant %+v", got, base)
		}
	})

	t.Run("non-nil scalars overwrite, including explicit zero", func(t *testing.T) {
		got := PartialConfig{
			Tag:         new(""), // an explicit zero still overwrites.
			VMName:      new("new-vm"),
			ArtifactDir: new("new-dir"),
		}.Apply(base)
		if got.Tag != "" {
			t.Errorf("Tag = %q, want explicit empty override", got.Tag)
		}
		if got.VMName != "new-vm" || got.ArtifactDir != "new-dir" {
			t.Errorf("scalar overwrite failed: %+v", got)
		}
	})
}

// clearConfigEnv unsets every config variable so a test's env layer is fully
// controlled. t.Setenv("") is not enough — LookupEnv still reports it present —
// so it is followed by Unsetenv; the t.Setenv registers restoration of the
// pre-test value on cleanup.
func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		envConfVar,
		"PODMAN_STATIC_DIST_TAG",
		"PODMAN_STATIC_DIST_VM_NAME",
		"PODMAN_STATIC_DIST_ARTIFACT_DIR",
	} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
}

func TestLoadConfigMissingFileYieldsDefaults(t *testing.T) {
	clearConfigEnv(t)
	missing := filepath.Join(t.TempDir(), "nope.json")
	cfg, err := LoadConfig(missing)
	if err != nil {
		t.Fatalf("missing config file must not error: %v", err)
	}
	if !reflect.DeepEqual(cfg, DefaultConfig()) {
		t.Errorf("cfg = %+v, want defaults %+v", cfg, DefaultConfig())
	}
}

func TestLoadConfigFileLayer(t *testing.T) {
	clearConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(
		path,
		[]byte(`{"tag":"file-tag","vm_name":"file-vm"}`),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Tag != "file-tag" || cfg.VMName != "file-vm" {
		t.Errorf("file layer not applied: %+v", cfg)
	}
	// a field absent from the file keeps its default.
	if cfg.ArtifactDir != DefaultConfig().ArtifactDir {
		t.Errorf("ArtifactDir = %q, want default", cfg.ArtifactDir)
	}
}

func TestLoadConfigEnvOverridesFile(t *testing.T) {
	clearConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(
		path,
		[]byte(`{"tag":"file-tag","vm_name":"file-vm"}`),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PODMAN_STATIC_DIST_TAG", "env-tag")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	// defaults < file < env: env wins over file.
	if cfg.Tag != "env-tag" {
		t.Errorf("Tag = %q, env should win over file", cfg.Tag)
	}
	// a field set only in the file (absent from env) keeps the file value.
	if cfg.VMName != "file-vm" {
		t.Errorf("VMName = %q, file value should survive when env is absent", cfg.VMName)
	}
}

func TestLoadConfigFileParseError(t *testing.T) {
	clearConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Error("LoadConfig should propagate a malformed-config parse error")
	}
}

func TestConfigPathFlagWins(t *testing.T) {
	t.Setenv(envConfVar, "/from/env.json")
	got, err := configPath("/from/flag.json")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/from/flag.json" {
		t.Errorf("configPath = %q, want the --config flag value", got)
	}
}

func TestConfigPathEnvWins(t *testing.T) {
	t.Setenv(envConfVar, "/from/env.json")
	got, err := configPath("")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/from/env.json" {
		t.Errorf("configPath = %q, want the $%s value", got, envConfVar)
	}
}

func TestConfigPathDefault(t *testing.T) {
	// The env override must be genuinely unset: Setenv("") still LookupEnv-succeeds.
	t.Setenv(envConfVar, "")
	os.Unsetenv(envConfVar)
	// Pin UserConfigDir deterministically (honored on Linux; ignored elsewhere,
	// which is fine because `want` is derived from the same call).
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "cfg"))

	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "devenv", "build", "podman-static", "config.json")

	got, err := configPath("")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("configPath = %q, want %q", got, want)
	}
}
