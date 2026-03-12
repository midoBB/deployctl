package cmd

import (
	"fmt"
	"os"
	"strings"

	"deployctl/internal"
	"github.com/spf13/cobra"
)

type validationCheck struct {
	Key     string
	OK      bool
	Message string
}

func newValidateCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "validate <app>",
		Short: "Run pre-flight checks",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options := currentCommandOptions(cmd)
			app := args[0]

			checks := []validationCheck{
				{Key: "container_file"},
				{Key: "image_parseable"},
				{Key: "image_pullable"},
				{Key: "systemd_unit"},
				{Key: "network_dependency"},
				{Key: "environment_file"},
			}

			containerPath := internal.ContainerFilePath(options.ContainerDir, app)
			containerExists := false
			info, statErr := os.Stat(containerPath)
			switch {
			case statErr == nil && !info.IsDir():
				checks[0].OK = true
				containerExists = true
			case statErr == nil && info.IsDir():
				checks[0].OK = false
				checks[0].Message = "container path is a directory"
			default:
				checks[0].OK = false
				checks[0].Message = statErr.Error()
			}

			var (
				containerFile *internal.ContainerFile
				parseErr      error
			)
			if containerExists {
				containerFile, parseErr = internal.LoadContainerFile(containerPath)
			} else {
				parseErr = fmt.Errorf("container file unavailable")
			}

			if parseErr == nil {
				hasContainerName := strings.TrimSpace(containerFile.ContainerName) != ""
				hasPublishPort := len(containerFile.PublishPorts) > 0
				if hasContainerName && hasPublishPort {
					checks[1].OK = true
				} else {
					checks[1].OK = false
					if !hasContainerName && !hasPublishPort {
						checks[1].Message = "missing ContainerName and PublishPort"
					} else if !hasContainerName {
						checks[1].Message = "missing ContainerName"
					} else {
						checks[1].Message = "missing PublishPort"
					}
				}
			} else {
				checks[1].OK = false
				checks[1].Message = "container file is not parseable"
			}

			podman := internal.Podman{}
			if parseErr == nil {
				if err := podman.ManifestInspect(cmd.Context(), containerFile.Image); err != nil {
					checks[2].OK = false
					checks[2].Message = err.Error()
				} else {
					checks[2].OK = true
				}
			} else {
				checks[2].OK = false
				checks[2].Message = "image unavailable because parsing failed"
			}

			systemd := internal.Systemd{}
			loaded, err := systemd.IsUnitLoaded(cmd.Context(), app)
			if err != nil {
				checks[3].OK = false
				checks[3].Message = err.Error()
			} else if !loaded {
				checks[3].OK = false
				checks[3].Message = "service unit is not loaded"
			} else {
				checks[3].OK = true
			}

			if parseErr == nil {
				networkUnits := internal.NetworkDependencyUnits(containerFile.Requires)
				if len(networkUnits) == 0 {
					checks[4].OK = false
					checks[4].Message = "no network dependency found in Requires="
				} else {
					activeChecks := make([]string, 0, len(networkUnits))
					allActive := true
					for _, unit := range networkUnits {
						state, activeErr := systemd.IsUnitActive(cmd.Context(), unit)
						if activeErr != nil {
							allActive = false
							activeChecks = append(activeChecks, fmt.Sprintf("%s=%v", unit, activeErr))
							continue
						}
						if state != "active" {
							allActive = false
							activeChecks = append(activeChecks, fmt.Sprintf("%s=%s", unit, state))
						}
					}

					if allActive {
						checks[4].OK = true
					} else {
						checks[4].OK = false
						checks[4].Message = strings.Join(activeChecks, ", ")
					}
				}
			} else {
				checks[4].OK = false
				checks[4].Message = "network dependencies unavailable because parsing failed"
			}

			if parseErr == nil {
				if len(containerFile.EnvironmentFiles) == 0 {
					checks[5].OK = false
					checks[5].Message = "no EnvironmentFile= entry found"
				} else {
					missing := make([]string, 0)
					for _, entry := range containerFile.EnvironmentFiles {
						resolved, resolveErr := internal.ResolveEnvironmentFilePath(entry)
						if resolveErr != nil {
							missing = append(missing, fmt.Sprintf("%s (%v)", entry, resolveErr))
							continue
						}
						if fileErr := internal.IsReadableFile(resolved); fileErr != nil {
							missing = append(missing, fmt.Sprintf("%s (%v)", resolved, fileErr))
						}
					}

					if len(missing) == 0 {
						checks[5].OK = true
					} else {
						checks[5].OK = false
						checks[5].Message = strings.Join(missing, ", ")
					}
				}
			} else {
				checks[5].OK = false
				checks[5].Message = "environment files unavailable because parsing failed"
			}

			allPassed := true
			fields := make([]internal.Field, 0, len(checks)+1)
			for _, check := range checks {
				status := "ok"
				if !check.OK {
					status = "failed"
					allPassed = false
				}
				fields = append(fields, internal.Field{Key: check.Key, Value: status})

				if !check.OK && check.Message != "" {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s: %s\n", check.Key, check.Message)
				}
			}

			validationStatus := "failed"
			if allPassed {
				validationStatus = "passed"
			}
			fields = append(fields, internal.Field{Key: "validation", Value: validationStatus})

			if err := internal.PrintOutput(cmd.OutOrStdout(), options.JSONOutput, fields); err != nil {
				return internal.NewExitError(1, err)
			}

			if allPassed {
				return nil
			}

			return internal.NewExitError(1, nil)
		},
	}

	return command
}
