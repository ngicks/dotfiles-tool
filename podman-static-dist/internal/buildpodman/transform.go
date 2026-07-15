package buildpodman

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// InterpolateTree rewrites every regular file under dir in place, expanding
// ${HOME} and ${XDG_DATA_HOME} (see InterpEnv). Files without those tokens are
// left byte-for-byte unchanged, so it is safe to run over the whole etc/containers
// tree (upstream policy.json, registries.conf, seccomp.json, ... pass through).
func InterpolateTree(ctx context.Context, dir string, env InterpEnv) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		if !d.Type().IsRegular() {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		expanded := env.Expand(string(b))
		if expanded == string(b) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(path, []byte(expanded), info.Mode().Perm())
	})
}

// transformUserUnit ports insert_environment_file.ts: on ExecStart/ExecStop
// lines it replaces the bare `podman` command with podmanPath, and before every
// ExecStart= line it inserts an EnvironmentFile= directive pointing at path.env.
func transformUserUnit(content, envFile, podmanPath string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines)+len(lines)/8+1)
	for _, l := range lines {
		if strings.HasPrefix(l, "ExecStart") || strings.HasPrefix(l, "ExecStop") {
			l = strings.ReplaceAll(l, "podman", podmanPath)
		}
		if strings.HasPrefix(l, "ExecStart=") {
			out = append(out, "EnvironmentFile="+envFile)
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// TransformUserUnitsInDir applies transformUserUnit to every regular file in dir.
func TransformUserUnitsInDir(dir, envFile, podmanPath string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		path := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := e.Info()
		if err != nil {
			return err
		}
		transformed := transformUserUnit(string(b), envFile, podmanPath)
		if err := os.WriteFile(path, []byte(transformed), info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}
