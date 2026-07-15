package buildpodman

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const podmanService = `[Unit]
Description=Podman API Service
Requires=podman.socket

[Service]
Type=exec
Environment=LOGGING="--log-level=info"
ExecStart=podman $LOGGING system service

[Install]
WantedBy=default.target
`

const podmanRestart = `[Service]
ExecStart=podman $LOGGING start --all --filter restart-policy=always
ExecStop=/bin/sh -c 'podman $LOGGING stop $(podman container ls --filter restart-policy=always -q)'
`

func TestTransformUserUnit_Service(t *testing.T) {
	const podmanPath = "/home/u/.local/containers/bin/podman"
	const envFile = "/home/u/.config/containers/path.env"
	got := transformUserUnit(podmanService, envFile, podmanPath)

	if !strings.Contains(
		got,
		"EnvironmentFile="+envFile+"\nExecStart="+podmanPath+" $LOGGING system service",
	) {
		t.Fatalf("EnvironmentFile not inserted before rewritten ExecStart:\n%s", got)
	}
	// the bare command is rewritten...
	if !strings.Contains(got, "ExecStart="+podmanPath+" $LOGGING system service") {
		t.Errorf("ExecStart podman path not rewritten:\n%s", got)
	}
	// ...but non-Exec lines (Requires=podman.socket) are untouched.
	if !strings.Contains(got, "Requires=podman.socket") {
		t.Errorf("non-Exec line was modified:\n%s", got)
	}
}

func TestTransformUserUnit_RewritesBothPodmanInExecStop(t *testing.T) {
	const podmanPath = "/p/podman"
	got := transformUserUnit(podmanRestart, "/e", podmanPath)
	// both `podman` occurrences on the ExecStop line are rewritten.
	if strings.Count(got, "podman") != strings.Count(got, "/p/podman") {
		t.Errorf("not every podman on Exec* lines rewritten:\n%s", got)
	}
	if strings.Contains(got, "'podman ") || strings.Contains(got, "$(podman ") {
		t.Errorf("bare podman survived on ExecStop:\n%s", got)
	}
	// one EnvironmentFile inserted for the single ExecStart=.
	if strings.Count(got, "EnvironmentFile=/e") != 1 {
		t.Errorf("want exactly one EnvironmentFile, got:\n%s", got)
	}
}

func TestInterpolateTree(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"storage.conf": "graphroot = ${XDG_DATA_HOME}/g\nrunroot = \"$XDG_RUNTIME_DIR/x\"\n",
		"path.sh":      "PATH=${HOME}/bin:${PATH}\n",
		"policy.json":  "{\"default\":[]}\n", // no tokens -> untouched
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	env := InterpEnv{Home: "/home/u", XdgDataHome: "/home/u/.local/share"}
	if err := InterpolateTree(context.Background(), dir, env); err != nil {
		t.Fatal(err)
	}

	if got := readf(
		t,
		filepath.Join(dir, "storage.conf"),
	); got != "graphroot = /home/u/.local/share/g\nrunroot = \"$XDG_RUNTIME_DIR/x\"\n" {
		t.Errorf("storage.conf = %q; XDG_DATA_HOME expanded, XDG_RUNTIME_DIR kept", got)
	}
	if got := readf(t, filepath.Join(dir, "path.sh")); got != "PATH=/home/u/bin:${PATH}\n" {
		t.Errorf("path.sh = %q; HOME expanded, PATH kept", got)
	}
	if got := readf(t, filepath.Join(dir, "policy.json")); got != "{\"default\":[]}\n" {
		t.Errorf("policy.json changed: %q", got)
	}
}

func readf(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
