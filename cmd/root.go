package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"deployctl/internal"
	"github.com/spf13/cobra"
)

type rootOptions struct {
	containerDir string
	jsonOutput   bool
}

type commandOptions struct {
	ContainerDir string
	JSONOutput   bool
}

const defaultContainerDir = "~/.config/containers/systemd"

type commandOptionsContextKey struct{}

func newRootCmd(version string) *cobra.Command {
	opts := rootOptions{containerDir: defaultContainerDir}

	rootCmd := &cobra.Command{
		Use:     "deployctl",
		Short:   "Deploy and manage systemd quadlet apps",
		Version: version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if os.Geteuid() == 0 {
				fmt.Fprintln(os.Stderr, "warning: deployctl should run as a non-root user in systemd user scope")
			}

			expanded, err := internal.ExpandPath(opts.containerDir)
			if err != nil {
				return internal.NewExitError(1, fmt.Errorf("invalid --container-dir: %w", err))
			}

			cmd.SetContext(withCommandOptions(cmd.Context(), commandOptions{
				ContainerDir: expanded,
				JSONOutput:   opts.jsonOutput,
			}))
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&opts.containerDir, "container-dir", opts.containerDir, "container file directory")
	rootCmd.PersistentFlags().BoolVar(&opts.jsonOutput, "json", false, "output JSON")

	rootCmd.AddCommand(newDeployCmd())
	rootCmd.AddCommand(newRollbackCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newValidateCmd())

	return rootCmd
}

func Execute(version string) int {
	err := newRootCmd(version).Execute()
	if err == nil {
		return 0
	}

	var exitErr *internal.ExitError
	if errors.As(err, &exitErr) {
		if exitErr.Err != nil {
			fmt.Fprintln(os.Stderr, exitErr.Err)
		}
		return exitErr.Code
	}

	fmt.Fprintln(os.Stderr, err)
	return 1
}

func withCommandOptions(ctx context.Context, options commandOptions) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, commandOptionsContextKey{}, options)
}

func currentCommandOptions(cmd *cobra.Command) commandOptions {
	if cmd != nil {
		ctx := cmd.Context()
		if ctx != nil {
			if options, ok := ctx.Value(commandOptionsContextKey{}).(commandOptions); ok {
				return options
			}
		}
	}

	return commandOptions{
		ContainerDir: defaultContainerDir,
		JSONOutput:   false,
	}
}
