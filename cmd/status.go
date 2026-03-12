package cmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"deployctl/internal"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "status <app>",
		Short: "Show deployment and runtime status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options := currentCommandOptions(cmd)
			app := args[0]

			containerPath := internal.ContainerFilePath(options.ContainerDir, app)
			containerFile, err := internal.LoadContainerFile(containerPath)
			if err != nil {
				return internal.NewExitError(1, fmt.Errorf("status read container file: %w", err))
			}

			previousImage := ""
			statePath, err := internal.StateFilePath(app)
			state, err := internal.ReadState(statePath)
			if err == nil {
				previousImage = state.PreviousImage
			} else if !errors.Is(err, os.ErrNotExist) {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning=state_read_failed app=%s message=%v\n", app, err)
			}

			systemd := internal.Systemd{}
			serviceState, err := systemd.IsActive(cmd.Context(), app)
			if err != nil {
				serviceState = "unknown"
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning=service_state_unavailable app=%s message=%v\n", app, err)
			}

			podman := internal.Podman{}
			healthState, err := podman.InspectHealth(cmd.Context(), app)
			if err != nil {
				healthState = ""
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning=health_state_unavailable app=%s message=%v\n", app, err)
			}

			uptime := resolveUptime(cmd, systemd, app)

			fields := []internal.Field{
				{Key: "app", Value: app},
				{Key: "image", Value: containerFile.Image},
				{Key: "previous_image", Value: previousImage},
				{Key: "service_state", Value: serviceState},
				{Key: "health_state", Value: healthState},
				{Key: "uptime", Value: uptime},
			}
			if err := internal.PrintOutput(cmd.OutOrStdout(), options.JSONOutput, fields); err != nil {
				return internal.NewExitError(1, err)
			}

			if serviceState == "active" && healthState == define.HealthCheckHealthy {
				return nil
			}

			return internal.NewExitError(1, nil)
		},
	}

	return command
}

func resolveUptime(cmd *cobra.Command, systemd internal.SystemdRunner, app string) string {
	usecValue, err := systemd.ShowProperty(cmd.Context(), app, "ActiveEnterTimestampUSec")
	if err == nil {
		startedAt, parseErr := internal.ParseSystemdUSec(usecValue)
		if parseErr == nil {
			return internal.FormatUptime(time.Since(startedAt))
		}
	}

	rawTimestamp, err := systemd.ShowProperty(cmd.Context(), app, "ActiveEnterTimestamp")
	if err == nil {
		startedAt, parseErr := internal.ParseSystemdTimestamp(rawTimestamp)
		if parseErr == nil {
			return internal.FormatUptime(time.Since(startedAt))
		}
	}

	return ""
}
