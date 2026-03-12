package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"deployctl/internal"
	"github.com/spf13/cobra"
)

const (
	stageValidateInputs = "validate_inputs"
	stageParseContainer = "parse_container_file"
	stageWriteState     = "write_state"
	stageUpdateFile     = "update_container_file"
	stageDaemonReload   = "daemon_reload"
	stageRestartService = "restart_service"
	stageHealthCheck    = "health_check"
	stageReadState      = "read_state"
)

type deployRequest struct {
	App          string
	Image        string
	ContainerDir string
	Timeout      time.Duration
	DryRun       bool
}

type deployResult struct {
	App           string
	Image         string
	PreviousImage string
	Noop          bool
	DryRun        bool
}

func newDeployCmd() *cobra.Command {
	var timeout time.Duration
	var dryRun bool

	command := &cobra.Command{
		Use:   "deploy <app> <image>",
		Short: "Deploy a new image",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			options := currentCommandOptions(cmd)
			request := deployRequest{
				App:          args[0],
				Image:        args[1],
				ContainerDir: options.ContainerDir,
				Timeout:      timeout,
				DryRun:       dryRun,
			}

			result, stageErr := runDeployWorkflow(cmd.Context(), request)
			if stageErr != nil {
				return emitOperationFailure(cmd, options, "deploy", request.App, stageErr)
			}

			switch {
			case result.Noop:
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "app %s already uses image %s; nothing to do\n", request.App, request.Image)
			case result.DryRun:
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "dry-run: would deploy app %s from %s to %s\n", request.App, result.PreviousImage, request.Image)
			default:
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "deployed app %s from %s to %s\n", request.App, result.PreviousImage, request.Image)
			}

			fields := []internal.Field{
				{Key: "deploy", Value: "ok"},
				{Key: "app", Value: request.App},
				{Key: "image", Value: request.Image},
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

func runDeployWorkflow(ctx context.Context, request deployRequest) (deployResult, *internal.StageError) {
	result := deployResult{
		App:    request.App,
		Image:  request.Image,
		DryRun: request.DryRun,
	}

	containerPath := internal.ContainerFilePath(request.ContainerDir, request.App)
	if err := validateDeployInputs(containerPath, request.Image); err != nil {
		return result, internal.NewStageError(stageValidateInputs, err)
	}

	containerFile, err := internal.LoadContainerFile(containerPath)
	if err != nil {
		return result, internal.NewStageError(stageParseContainer, err)
	}

	result.PreviousImage = containerFile.Image
	if containerFile.Image == request.Image {
		result.Noop = true
		return result, nil
	}

	if request.DryRun {
		return result, nil
	}

	statePath, err := internal.StateFilePath(request.App)
	if err != nil {
		return result, internal.NewStageError(stageReadState, err)
	}
	existingState, err := internal.ReadState(statePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return result, internal.NewStageError(stageReadState, err)
	}

	lastGoodImage := containerFile.Image
	lastGoodAt := ""
	if existingState != nil {
		if rollbackImage := existingState.RollbackImage(); rollbackImage != "" {
			lastGoodImage = rollbackImage
		}
		lastGoodAt = existingState.LastGoodAt
	}

	deployedAt := time.Now().UTC().Format(time.RFC3339)
	state := internal.StateRecord{
		App:           request.App,
		PreviousImage: containerFile.Image,
		DeployedImage: request.Image,
		DeployedAt:    deployedAt,
		LastGoodImage: lastGoodImage,
		LastGoodAt:    lastGoodAt,
	}
	if state.LastGoodAt == "" && state.LastGoodImage != "" {
		state.LastGoodAt = deployedAt
	}
	if err := internal.WriteStateAtomic(statePath, state); err != nil {
		return result, internal.NewStageError(stageWriteState, err)
	}

	if err := internal.UpdateContainerImageAtomic(containerFile, request.Image); err != nil {
		return result, internal.NewStageError(stageUpdateFile, err)
	}

	systemd := internal.Systemd{}
	if err := systemd.DaemonReload(ctx); err != nil {
		return result, internal.NewStageError(stageDaemonReload, err)
	}

	if err := systemd.RestartService(ctx, request.App); err != nil {
		stageErr := internal.NewStageError(stageRestartService, err)
		attachJournal(ctx, systemd, request.App, stageErr)
		return result, stageErr
	}

	if request.Timeout <= 0 {
		request.Timeout = 60 * time.Second
	}

	podman := internal.Podman{}
	if _, err := internal.WaitForHealthy(ctx, &systemd, &podman, request.App, request.Timeout, 2*time.Second); err != nil {
		stageErr := internal.NewStageError(stageHealthCheck, err)
		attachJournal(ctx, systemd, request.App, stageErr)
		return result, stageErr
	}

	state.LastGoodImage = request.Image
	state.LastGoodAt = time.Now().UTC().Format(time.RFC3339)
	state.DeployedAt = state.LastGoodAt
	if err := internal.WriteStateAtomic(statePath, state); err != nil {
		return result, internal.NewStageError(stageWriteState, err)
	}

	return result, nil
}

func validateDeployInputs(containerPath, image string) error {
	if strings.TrimSpace(image) == "" {
		return fmt.Errorf("image cannot be empty")
	}
	if !looksLikeImageReference(image) {
		return fmt.Errorf("image reference %q does not look valid", image)
	}

	info, err := os.Stat(containerPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("container file %s does not exist", containerPath)
		}
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("container path %s is a directory", containerPath)
	}

	return nil
}

func looksLikeImageReference(image string) bool {
	slashIdx := strings.Index(image, "/")
	if slashIdx < 0 {
		return false
	}
	afterSlash := image[slashIdx+1:]
	return strings.Contains(afterSlash, ":") || strings.Contains(image, "@sha256:")
}

func emitOperationFailure(cmd *cobra.Command, options commandOptions, operation, app string, stageErr *internal.StageError) error {
	fields := []internal.Field{
		{Key: operation, Value: "failed"},
		{Key: "app", Value: app},
		{Key: "stage", Value: stageErr.Stage},
	}
	if err := internal.PrintOutput(cmd.OutOrStdout(), options.JSONOutput, fields); err != nil {
		return internal.NewExitError(1, err)
	}

	if stageErr.Err != nil {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), stageErr.Err)
	}
	if stageErr.Journal != "" {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), stageErr.Journal)
	}

	return internal.NewExitError(1, nil)
}

func attachJournal(ctx context.Context, systemd internal.SystemdRunner, app string, stageErr *internal.StageError) {
	journal, err := systemd.JournalTail(ctx, app, 30)
	if journal != "" {
		stageErr.Journal = journal
	}
	if err != nil {
		if stageErr.Err != nil {
			stageErr.Err = fmt.Errorf("%w; failed to read journal: %v", stageErr.Err, err)
		} else {
			stageErr.Err = fmt.Errorf("failed to read journal: %w", err)
		}
	}
}
