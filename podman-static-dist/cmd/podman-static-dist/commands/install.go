package commands

import (
	"github.com/spf13/cobra"

	"github.com/ngicks/dotfiles-tool/podman-static-dist/pkg/podmanstaticdist"
)

func installCmd(parent *cobra.Command, flagConfig *string) {
	var (
		flagTar string
		flagTag string
	)

	cmd := &cobra.Command{
		Use:               "install",
		Short:             "Extract an artifact and wire podman-static into your home",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstall(cmd, args, *flagConfig, flagTar, flagTag)
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

func runInstall(
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

	return podmanstaticdist.New(cfg).Install(cmd.Context(), podmanstaticdist.InstallParams{
		ExtractParams: podmanstaticdist.ExtractParams{
			TarPath: flagTar,
			Tag:     tag,
		},
	})
}
