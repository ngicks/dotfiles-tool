package commands

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/ngicks/dotfiles-tool/podman-static-dist/pkg/podmanstaticdist"
	"github.com/ngicks/dotfiles-tool/podman-static-dist/pkg/podmanstaticdist/cli"
)

func buildCmd(parent *cobra.Command, flagConfig *string) {
	var (
		flagOutput   string
		flagTag      string
		flagVMName   string
		flagWork     string
		flagRecreate bool
		flagYes      bool
	)

	cmd := &cobra.Command{
		Use:               "build",
		Short:             "Build the static podman distribution artifact in a Lima VM",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBuild(
				cmd, args, *flagConfig,
				flagOutput, flagTag, flagVMName, flagWork, flagRecreate, flagYes,
			)
		},
	}

	f := cmd.Flags()
	f.StringVarP(
		&flagOutput,
		"output",
		"o",
		"",
		"output .tar.zst path "+
			"(default: <user cache dir>/dotfiles/build/podman-static/out/podman-static-<tag>.tar.zst)",
	)
	f.StringVar(&flagTag, "tag", "", "podman-static tag to build (default: config/embedded)")
	f.StringVar(&flagVMName, "vm-name", "", "Lima instance name (default: config/lima)")
	f.StringVar(
		&flagWork,
		"work",
		"",
		"host work dir shared with the VM (default: <user cache dir>/dotfiles/build/podman-static)",
	)
	f.BoolVar(&flagRecreate, "recreate", false, "recreate the Lima VM before building")
	f.BoolVar(&flagYes, "yes", false, "do not prompt before creating the VM")

	parent.AddCommand(cmd)
}

func runBuild(
	cmd *cobra.Command, _ []string, flagConfig string,
	flagOutput, flagTag, flagVMName, flagWork string, flagRecreate, flagYes bool,
) error {
	cfg, err := podmanstaticdist.LoadConfig(flagConfig)
	if err != nil {
		return err
	}
	overlayBuildFlags(cmd, &cfg, flagTag, flagVMName)

	var confirm func(prompt string) (bool, error)
	if !flagYes {
		confirm = func(prompt string) (bool, error) {
			return cli.Confirm(os.Stdin, os.Stderr, prompt)
		}
	}

	return podmanstaticdist.New(cfg).Build(cmd.Context(), podmanstaticdist.BuildParams{
		OutputPath: flagOutput,
		Work:       flagWork,
		Recreate:   flagRecreate,
		Confirm:    confirm,
	})
}

func overlayBuildFlags(
	cmd *cobra.Command,
	cfg *podmanstaticdist.Config,
	flagTag, flagVMName string,
) {
	if cmd.Flags().Changed("tag") {
		cfg.Tag = flagTag
	}
	if cmd.Flags().Changed("vm-name") {
		cfg.VMName = flagVMName
	}
}
