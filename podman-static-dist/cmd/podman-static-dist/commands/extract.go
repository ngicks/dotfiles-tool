package commands

import (
	"github.com/spf13/cobra"

	"github.com/ngicks/dotfiles-tool/podman-static-dist/pkg/podmanstaticdist"
)

const extractLong = `extract unpacks the .tar.zst artifact into the versioned dist dir
($TARGET_ARTIFACT_DIR/<tag>, else the config's artifact_dir/<tag>, else
${XDG_DATA_HOME:-$HOME/.local/share}/podman-dist/<tag>) and interpolates the
bundled config against your environment. It leaves your home untouched: no
symlinks are created, no systemd wiring or daemon-reload is run.

The tree is extracted as-is: environment-specific config sets shipped under
etc/containers/__additional_<name>/ (e.g. __additional_podman-in-podman for
podman inside the devenv container) are kept in place; a consumer activates one
by bind-mounting its files over their default-named counterparts (the devenv
runner shadows ~/.config/containers/{containers,storage}.conf and path.sh).

Use install for the full flow (extract + symlink wiring) or link to wire an
already-extracted tree into your home.`

func extractCmd(parent *cobra.Command, flagConfig *string) {
	var (
		flagTar string
		flagTag string
	)

	cmd := &cobra.Command{
		Use:               "extract",
		Short:             "Unpack an artifact into the dist dir and interpolate config (no symlinks)",
		Long:              extractLong,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExtract(cmd, args, *flagConfig, flagTar, flagTag)
		},
	}

	f := cmd.Flags()
	f.StringVar(&flagTar, "tar", "", "path to the .tar.zst artifact (required)")
	f.StringVar(
		&flagTag,
		"tag",
		"",
		"install tag / subdir name (default: archive stamp, else config/embedded)",
	)
	_ = cmd.MarkFlagRequired("tar")

	parent.AddCommand(cmd)
}

func runExtract(
	cmd *cobra.Command, _ []string, flagConfig, flagTar, flagTag string,
) error {
	cfg, err := podmanstaticdist.LoadConfig(flagConfig)
	if err != nil {
		return err
	}
	var tag string
	if cmd.Flags().Changed("tag") {
		tag = flagTag
	}

	return podmanstaticdist.New(cfg).Extract(cmd.Context(), podmanstaticdist.ExtractParams{
		TarPath: flagTar,
		Tag:     tag,
	})
}
