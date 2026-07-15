package buildpodman

import "testing"

func lookup(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

func TestResolveDefaultsXdgDataHome(t *testing.T) {
	env, err := resolveInterpEnv(lookup(map[string]string{"HOME": "/home/u"}))
	if err != nil {
		t.Fatal(err)
	}
	if env.XdgDataHome != "/home/u/.local/share" {
		t.Errorf("XdgDataHome = %q, want default", env.XdgDataHome)
	}
}

func TestResolveHonorsXdgDataHome(t *testing.T) {
	env, err := resolveInterpEnv(
		lookup(map[string]string{"HOME": "/home/u", "XDG_DATA_HOME": "/data"}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if env.XdgDataHome != "/data" {
		t.Errorf("XdgDataHome = %q, want /data", env.XdgDataHome)
	}
}

func TestResolveRequiresHome(t *testing.T) {
	if _, err := resolveInterpEnv(lookup(map[string]string{})); err == nil {
		t.Error("Resolve should fail without HOME")
	}
}

func TestExpandOnlyTouchesBracedHomeAndDataHome(t *testing.T) {
	env := InterpEnv{Home: "/home/u", XdgDataHome: "/home/u/.local/share"}
	cases := []struct{ in, want string }{
		// replaced
		{"${HOME}/.local/containers/bin", "/home/u/.local/containers/bin"},
		{"graphroot = ${XDG_DATA_HOME}/containers", "graphroot = /home/u/.local/share/containers"},
		// left intact — expand at session/runtime
		{
			`runroot = "$XDG_RUNTIME_DIR/containers/storage"`,
			`runroot = "$XDG_RUNTIME_DIR/containers/storage"`,
		},
		{"export PATH=\"$_c_bin:$PATH\"", "export PATH=\"$_c_bin:$PATH\""},
		{"${XDG_DATA_HOME:-$HOME/.local/share}/bin", "${XDG_DATA_HOME:-$HOME/.local/share}/bin"},
		{
			"${XDG_CONFIG_HOME:-$HOME/.config}/containers-quadlet",
			"${XDG_CONFIG_HOME:-$HOME/.config}/containers-quadlet",
		},
	}
	for _, c := range cases {
		if got := env.Expand(c.in); got != c.want {
			t.Errorf("Expand(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
