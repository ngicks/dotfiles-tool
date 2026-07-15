package buildpodman

import (
	"fmt"
	"strings"
)

// InterpEnv holds the values substituted into config files. It mirrors the old
// copy_conf_interpolating.ts behavior: only ${HOME} and ${XDG_DATA_HOME} are
// replaced (synthesizing XDG_DATA_HOME as $HOME/.local/share when unset); every
// other token ($XDG_RUNTIME_DIR, ${VAR:-default}, shell locals) is intentionally
// left intact so it expands at session/runtime time.
type InterpEnv struct {
	Home        string
	XdgDataHome string
}

// resolveInterpEnv builds an InterpEnv from a lookup (typically os.LookupEnv),
// defaulting XDG_DATA_HOME to $HOME/.local/share when unset. HOME is required.
func resolveInterpEnv(lookup func(string) (string, bool)) (InterpEnv, error) {
	home, ok := lookup("HOME")
	if !ok || home == "" {
		return InterpEnv{}, fmt.Errorf("environment variable HOME is not set")
	}
	xdg, ok := lookup("XDG_DATA_HOME")
	if !ok || xdg == "" {
		xdg = home + "/.local/share"
	}
	return InterpEnv{Home: home, XdgDataHome: xdg}, nil
}

// Expand replaces ${HOME} and ${XDG_DATA_HOME} in text. NewReplacer matches the
// exact braced tokens only, so bare $XDG_RUNTIME_DIR and defaulted
// ${XDG_DATA_HOME:-...} forms are left untouched.
func (e InterpEnv) Expand(text string) string {
	return strings.NewReplacer(
		"${HOME}", e.Home,
		"${XDG_DATA_HOME}", e.XdgDataHome,
	).Replace(text)
}
