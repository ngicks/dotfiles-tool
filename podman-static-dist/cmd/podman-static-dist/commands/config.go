package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ngicks/dotfiles-tool/podman-static-dist/pkg/podmanstaticdist"
	"github.com/ngicks/dotfiles-tool/podman-static-dist/pkg/podmanstaticdist/cli"
)

const configLongFmt = `config loads every layer (defaults < file < environment), applies any
explicitly-set flags on top, and prints the fully-resolved configuration. With
no flags it prints indented JSON; with --format it renders a Go text/template
against the config value instead.

The config file is JSON. Its location resolves as: --config flag, else
$PODMAN_STATIC_DIST_CONF, else os.UserConfigDir()/devenv/build/podman-static/config.json.
Note this default deliberately differs from the project-name convention
(UserConfigDir()/podman-static-dist): the tool ships inside the devenv build tree,
so its config lives beside the other devenv build assets.

The value passed to --format has this shape (Go field name, type, JSON key);
nesting is shown as a tree so deep configs stay readable:

  Config
  ├─ .Tag          string    # podman-static tag to build/install  (tag)
  ├─ .VMName       string    # Lima instance name (build)          (vm_name)
  └─ .ArtifactDir  string    # dist base dir override              (artifact_dir)

Use the Go field names in --format (e.g. {{.Tag}}); the default JSON output uses
the lower-case keys shown in parentheses. The template also sees these helper
functions:

%s`

const configExample = `  podman-static-dist config
  podman-static-dist config --format '{{.Tag}}'
  podman-static-dist config --format '{{ json . }}'`

func configCmd(parent *cobra.Command, flagConfig *string) {
	var flagFormat string

	cmd := &cobra.Command{
		Use:               "config",
		Short:             "Print the resolved configuration",
		Long:              fmt.Sprintf(configLongFmt, cli.TemplateFuncHelp()),
		Example:           configExample,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfig(cmd, args, *flagConfig, flagFormat)
		},
	}

	cmd.Flags().StringVarP(
		&flagFormat,
		"format",
		"f",
		"",
		"Go text/template rendered against the resolved config instead of JSON",
	)

	parent.AddCommand(cmd)
}

func runConfig(cmd *cobra.Command, _ []string, flagConfig, flagFormat string) error {
	cfg, err := podmanstaticdist.LoadConfig(flagConfig)
	if err != nil {
		return err
	}
	return cli.RenderConfig(cmd.OutOrStdout(), cfg, flagFormat)
}
