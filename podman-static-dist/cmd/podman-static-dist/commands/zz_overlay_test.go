package commands

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/ngicks/dotfiles-tool/podman-static-dist/pkg/podmanstaticdist"
)

func TestOverlayBuildFlags(t *testing.T) {
	buildFlags := func(t *testing.T, args []string) (*cobra.Command, string, string) {
		t.Helper()
		root := &cobra.Command{Use: "podman-static-dist"}
		var flagConfig string
		buildCmd(root, &flagConfig)

		cmd, _, err := root.Find([]string{"build"})
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.ParseFlags(args); err != nil {
			t.Fatal(err)
		}
		flagTag, err := cmd.Flags().GetString("tag")
		if err != nil {
			t.Fatal(err)
		}
		flagVMName, err := cmd.Flags().GetString("vm-name")
		if err != nil {
			t.Fatal(err)
		}
		return cmd, flagTag, flagVMName
	}

	t.Run("explicit --vm-name \"\" overwrites the config vm name", func(t *testing.T) {
		cmd, flagTag, flagVMName := buildFlags(t, []string{"--output", "/x", "--vm-name", ""})

		cfg := podmanstaticdist.Config{Tag: "from-defaults", VMName: "from-defaults"}
		overlayBuildFlags(cmd, &cfg, flagTag, flagVMName)

		if cfg.VMName != "" {
			t.Errorf("VMName = %q, want the explicit empty flag to win", cfg.VMName)
		}
		if cfg.Tag != "from-defaults" {
			t.Errorf("Tag = %q, want the loaded config value (--tag unset)", cfg.Tag)
		}
	})

	t.Run("unset flags leave the config untouched", func(t *testing.T) {
		cmd, flagTag, flagVMName := buildFlags(t, []string{"--output", "/x"})

		cfg := podmanstaticdist.Config{Tag: "from-defaults", VMName: "from-defaults"}
		overlayBuildFlags(cmd, &cfg, flagTag, flagVMName)

		if cfg.VMName != "from-defaults" || cfg.Tag != "from-defaults" {
			t.Errorf("config mutated by unset flags: %+v", cfg)
		}
	})
}
