// Package commands defines the podman-static-dist cobra command tree.
package commands

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/ngicks/go-common/contextkey"
	"github.com/spf13/cobra"

	"github.com/ngicks/dotfiles-tool/podman-static-dist/internal/loggerfactory"
)

func Execute(ctx context.Context) error {
	return rootCmd().ExecuteContext(ctx)
}

func rootCmd() *cobra.Command {
	var (
		logConfig   *loggerfactory.Config
		flagVersion bool
		flagConfig  string
	)

	cmd := &cobra.Command{
		Use:           "podman-static-dist",
		Short:         "Build and install a static podman distribution",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if err := loggerfactory.ReadEnv(
				logConfig,
				"podman-static-dist",
				os.Environ(),
			); err != nil {
				fmt.Fprintln(os.Stderr, "warning:", err)
			}
			logger := loggerfactory.BuildLogger(logConfig)
			slog.SetDefault(logger)
			cmd.SetContext(contextkey.WithSlogLogger(cmd.Context(), logger))
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagVersion {
				return runVersion(cmd, args)
			}
			return runRoot(cmd, args)
		},
	}

	logConfig = loggerfactory.RegisterFlags(cmd)
	cmd.Flags().BoolVar(&flagVersion, "version", false, "alias for the version subcommand")
	cmd.PersistentFlags().
		StringVar(&flagConfig, "config", "", "config file path; overrides the default location")

	versionCmd(cmd)
	configCmd(cmd, &flagConfig)
	buildCmd(cmd, &flagConfig)
	extractCmd(cmd, &flagConfig)
	installCmd(cmd, &flagConfig)
	linkCmd(cmd, &flagConfig)

	return cmd
}

func runRoot(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}
