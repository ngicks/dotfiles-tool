package commands

import (
	"github.com/spf13/cobra"

	"github.com/ngicks/dotfiles-tool/podman-static-dist/pkg/podmanstaticdist"
)

const linkLong = `link wires an already-extracted dist tree into your home without re-extracting
it. It targets the case where the dist dir is a READ-ONLY mount and/or $HOME
differs from the host that produced the tree (e.g. inside a container).

link only creates symlinks — no interpolation happens here (that is done once, at
extract time). It points ~/.config/containers at <base>/current/etc/containers,
per-file links the environment.d and systemd/user units (units and binaries are
addressed directly under <base>/current/usr/local; there is no home-side binary
link), and installs the quadlet generator. Re-running relinks; it is idempotent.

The base dir defaults to $TARGET_ARTIFACT_DIR, else the config's artifact_dir,
else $XDG_DATA_HOME/podman-dist. The current symlink is only created/updated
when --tag is given AND the base is writable; a read-only base with an existing
current symlink is used as-is.`

func linkCmd(parent *cobra.Command, flagConfig *string) {
	var (
		flagBase        string
		flagTag         string
		flagSkipSystemd bool
	)

	cmd := &cobra.Command{
		Use:               "link",
		Short:             "Wire an already-extracted dist tree into your home (read-only mount safe)",
		Long:              linkLong,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLink(cmd, args, *flagConfig, flagBase, flagTag, flagSkipSystemd)
		},
	}

	f := cmd.Flags()
	f.StringVar(&flagBase, "base", "", "dist base directory")
	f.StringVar(&flagTag, "tag", "", "tag for the current symlink")
	f.BoolVar(&flagSkipSystemd, "skip-systemd", false, "skip systemd wiring")

	parent.AddCommand(cmd)
}

func runLink(
	cmd *cobra.Command, _ []string, flagConfig,
	flagBase, flagTag string, flagSkipSystemd bool,
) error {
	cfg, err := podmanstaticdist.LoadConfig(flagConfig)
	if err != nil {
		return err
	}

	return podmanstaticdist.New(cfg).Link(cmd.Context(), podmanstaticdist.LinkParams{
		Base:        flagBase,
		Tag:         flagTag,
		SkipSystemd: flagSkipSystemd,
	})
}
