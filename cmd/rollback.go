package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"deployctl/internal"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/spf13/cobra"
)

func newRollbackCmd() *cobra.Command {
	var timeout time.Duration
	var dryRun bool

	command := &cobra.Command{
		Use:   "rollback <app>",
		Short: "Rollback to the last known good image",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options := currentCommandOptions(cmd)
			app := args[0]

			state, initialized, stageErr := loadOrInitializeRollbackState(cmd.Context(), app, options.ContainerDir, dryRun)
			if stageErr != nil {
				return emitOperationFailure(cmd, options, "rollback", app, stageErr)
			}

			rollbackImage := strings.TrimSpace(state.RollbackImage())
			if rollbackImage == "" {
				stageErr := internal.NewStageError(stageReadState, fmt.Errorf("no last good deployment recorded"))
				return emitOperationFailure(cmd, options, "rollback", app, stageErr)
			}

			if initialized {
				if dryRun {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "dry-run: would mark current healthy image %s as last good for app %s\n", rollbackImage, app)
				} else {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "marked current healthy image %s as last good for app %s\n", rollbackImage, app)
				}
			}

			request := deployRequest{
				App:          app,
				Image:        rollbackImage,
				ContainerDir: options.ContainerDir,
				Timeout:      timeout,
				DryRun:       dryRun,
			}

			result, stageErr := runDeployWorkflow(cmd.Context(), request)
			if stageErr != nil {
				return emitOperationFailure(cmd, options, "rollback", app, stageErr)
			}

			switch {
			case result.Noop:
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "app %s already uses last good image %s; nothing to do\n", app, rollbackImage)
			case result.DryRun:
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "dry-run: would rollback app %s from %s to %s\n", app, result.PreviousImage, rollbackImage)
			default:
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "rolled back app %s to %s\n", app, rollbackImage)
			}

			fields := []internal.Field{
				{Key: "rollback", Value: "ok"},
				{Key: "app", Value: app},
				{Key: "image", Value: rollbackImage},
			}
			if result.DryRun {
				fields = append(fields, internal.Field{Key: "dry_run", Value: true})
			}

			if err := internal.PrintOutput(cmd.OutOrStdout(), options.JSONOutput, fields); err != nil {
				return internal.NewExitError(1, err)
			}

			return nil
		},
	}

	command.Flags().DurationVar(&timeout, "timeout", 60*time.Second, "health check timeout")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "show what would change without applying it")

	return command
}

func loadOrInitializeRollbackState(ctx context.Context, app, containerDir string, dryRun bool) (*internal.StateRecord, bool, *internal.StageError) {
	statePath, err := internal.StateFilePath(app)
	if err != nil {
		return nil, false, internal.NewStageError(stageReadState, err)
	}
	state, err := internal.ReadState(statePath)
	if err == nil {
		return state, false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, false, internal.NewStageError(stageReadState, err)
	}

	containerPath := internal.ContainerFilePath(containerDir, app)
	containerFile, parseErr := internal.LoadContainerFile(containerPath)
	if parseErr != nil {
		return nil, false, internal.NewStageError(stageParseContainer, parseErr)
	}

	systemd := internal.Systemd{}
	serviceState, serviceErr := systemd.IsActive(ctx, app)
	if serviceErr != nil {
		return nil, false, internal.NewStageError(stageHealthCheck, fmt.Errorf("check service state: %w", serviceErr))
	}
	if serviceState != "active" {
		return nil, false, internal.NewStageError(stageHealthCheck, fmt.Errorf("service state is %q; cannot mark current image as last good", serviceState))
	}

	subState, subErr := systemd.ShowProperty(ctx, app, "SubState")
	if subErr != nil {
		return nil, false, internal.NewStageError(stageHealthCheck, fmt.Errorf("check service substate: %w", subErr))
	}
	if subState != "running" {
		return nil, false, internal.NewStageError(stageHealthCheck, fmt.Errorf("service substate is %q; cannot mark current image as last good", subState))
	}

	podman := internal.Podman{}
	healthState, healthErr := podman.InspectHealth(ctx, app)
	if healthErr != nil {
		return nil, false, internal.NewStageError(stageHealthCheck, fmt.Errorf("inspect container health: %w", healthErr))
	}
	if healthState != "" && healthState != define.HealthCheckHealthy {
		return nil, false, internal.NewStageError(stageHealthCheck, fmt.Errorf("container health is %q; cannot mark current image as last good", healthState))
	}

	now := time.Now().UTC().Format(time.RFC3339)
	state = &internal.StateRecord{
		App:           app,
		PreviousImage: containerFile.Image,
		DeployedImage: containerFile.Image,
		DeployedAt:    now,
		LastGoodImage: containerFile.Image,
		LastGoodAt:    now,
	}

	if !dryRun {
		if writeErr := internal.WriteStateAtomic(statePath, *state); writeErr != nil {
			return nil, false, internal.NewStageError(stageWriteState, writeErr)
		}
	}

	return state, true, nil
}
