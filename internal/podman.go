package internal

import (
	"context"
	"fmt"

	"github.com/containers/podman/v5/libpod/define"
)

// PodmanRunner abstracts podman operations for testability.
type PodmanRunner interface {
	PullQuiet(ctx context.Context, image string) error
	ManifestInspect(ctx context.Context, image string) error
	InspectHealth(ctx context.Context, container string) (string, error)
}

type Podman struct{}

func (Podman) PullQuiet(ctx context.Context, image string) error {
	result, err := RunCommand(ctx, "podman", "pull", "--quiet", image)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("podman pull --quiet %s failed: %s", image, failureDetails(result))
	}
	return nil
}

func (Podman) ManifestInspect(ctx context.Context, image string) error {
	result, err := RunCommand(ctx, "podman", "manifest", "inspect", image)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("podman manifest inspect %s failed: %s", image, failureDetails(result))
	}
	return nil
}

func (Podman) InspectHealth(ctx context.Context, container string) (string, error) {
	result, err := RunCommand(
		ctx,
		"podman",
		"container",
		"inspect",
		"--format",
		"{{if .State.Health}}{{.State.Health.Status}}{{end}}",
		container,
	)
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("podman container inspect %s failed: %s", container, failureDetails(result))
	}

	status := result.Stdout
	if status == "" {
		return "", nil
	}
	if status == define.HealthCheckHealthy || status == define.HealthCheckUnhealthy || status == define.HealthCheckStarting {
		return status, nil
	}

	return "", fmt.Errorf("unrecognized health status %q from podman container inspect %s", status, container)
}
