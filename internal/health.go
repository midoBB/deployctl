package internal

import (
	"context"
	"fmt"
	"time"

	"github.com/containers/podman/v5/libpod/define"
)

type HealthSnapshot struct {
	ServiceState string
	SubState     string
	HealthState  string
}

func WaitForHealthy(ctx context.Context, systemd SystemdRunner, podman PodmanRunner, app string, timeout, interval time.Duration) (HealthSnapshot, error) {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	if interval <= 0 {
		interval = 2 * time.Second
	}

	deadline := time.Now().Add(timeout)
	lastErr := fmt.Errorf("service did not report healthy state yet")
	snapshot := HealthSnapshot{}
	var prevServiceState string

	for {
		if time.Now().After(deadline) {
			return snapshot, fmt.Errorf("health verification timed out after %s: %w", timeout, lastErr)
		}

		serviceState, err := systemd.IsActive(ctx, app)
		if err != nil {
			return snapshot, fmt.Errorf("check systemd active state: %w", err)
		}
		snapshot.ServiceState = serviceState

		if serviceState == "inactive" {
			return snapshot, fmt.Errorf("service entered %q state", serviceState)
		}

		if serviceState == "failed" {
			if prevServiceState == "failed" {
				return snapshot, fmt.Errorf("service entered %q state (restart loop detected)", serviceState)
			}
			prevServiceState = "failed"
			lastErr = fmt.Errorf("service entered %q state", serviceState)
		} else {
			prevServiceState = serviceState

			subState, subErr := systemd.ShowProperty(ctx, app, "SubState")
			if subErr != nil {
				lastErr = fmt.Errorf("read service substate: %w", subErr)
			} else {
				snapshot.SubState = subState
			}

			healthState, healthErr := podman.InspectHealth(ctx, app)
			if healthErr != nil {
				lastErr = fmt.Errorf("inspect container health: %w", healthErr)
			} else {
				snapshot.HealthState = healthState
			}

			hasHealthCheck := healthErr == nil && healthState != ""
			if serviceState == "active" {
				if hasHealthCheck {
					if snapshot.SubState == "running" && healthState == define.HealthCheckHealthy {
						return snapshot, nil
					}
					lastErr = fmt.Errorf("waiting for running+healthy state (substate=%s health=%s)", snapshot.SubState, healthState)
				} else if healthErr == nil {
					return snapshot, nil
				}
			}
		}

		remaining := time.Until(deadline)
		waitFor := interval
		if remaining < waitFor {
			waitFor = remaining
		}
		if waitFor <= 0 {
			continue
		}

		timer := time.NewTimer(waitFor)
		select {
		case <-ctx.Done():
			timer.Stop()
			return snapshot, fmt.Errorf("health verification canceled: %w", ctx.Err())
		case <-timer.C:
		}
	}
}
