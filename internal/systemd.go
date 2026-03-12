package internal

import (
	"context"
	"fmt"
	"strings"
)

// SystemdRunner abstracts systemd operations for testability.
type SystemdRunner interface {
	DaemonReload(ctx context.Context) error
	RestartService(ctx context.Context, app string) error
	IsActive(ctx context.Context, app string) (string, error)
	IsUnitActive(ctx context.Context, unit string) (string, error)
	ShowProperty(ctx context.Context, app, property string) (string, error)
	ShowUnitProperty(ctx context.Context, unit, property string) (string, error)
	IsUnitLoaded(ctx context.Context, app string) (bool, error)
	JournalTail(ctx context.Context, app string, lines int) (string, error)
}

type Systemd struct{}

func ServiceUnitName(app string) string {
	return app + ".service"
}

func (Systemd) DaemonReload(ctx context.Context) error {
	result, err := RunCommand(ctx, "systemctl", "--user", "daemon-reload")
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("systemctl --user daemon-reload failed: %s", failureDetails(result))
	}
	return nil
}

func (Systemd) RestartService(ctx context.Context, app string) error {
	unit := ServiceUnitName(app)
	result, err := RunCommand(ctx, "systemctl", "--user", "restart", unit)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("systemctl --user restart %s failed: %s", unit, failureDetails(result))
	}
	return nil
}

func (s Systemd) IsActive(ctx context.Context, app string) (string, error) {
	return s.IsUnitActive(ctx, ServiceUnitName(app))
}

func (Systemd) IsUnitActive(ctx context.Context, unit string) (string, error) {
	result, err := RunCommand(ctx, "systemctl", "--user", "is-active", unit)
	if err != nil {
		return "", err
	}

	state := strings.TrimSpace(result.Stdout)
	if state == "" {
		state = strings.TrimSpace(result.Stderr)
	}
	if state == "" {
		state = "unknown"
	}

	return state, nil
}

func (s Systemd) ShowProperty(ctx context.Context, app, property string) (string, error) {
	return s.ShowUnitProperty(ctx, ServiceUnitName(app), property)
}

func (Systemd) ShowUnitProperty(ctx context.Context, unit, property string) (string, error) {
	result, err := RunCommand(ctx, "systemctl", "--user", "show", unit, "--property="+property, "--value")
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("systemctl --user show %s (%s) failed: %s", unit, property, failureDetails(result))
	}

	return strings.TrimSpace(result.Stdout), nil
}

func (Systemd) IsUnitLoaded(ctx context.Context, app string) (bool, error) {
	unit := ServiceUnitName(app)
	result, err := RunCommand(ctx, "systemctl", "--user", "list-unit-files", unit)
	if err != nil {
		return false, err
	}
	if result.ExitCode != 0 {
		return false, fmt.Errorf("systemctl --user list-unit-files %s failed: %s", unit, failureDetails(result))
	}

	for _, line := range strings.Split(result.Stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == unit {
			return true, nil
		}
	}

	return false, nil
}

func (Systemd) JournalTail(ctx context.Context, app string, lines int) (string, error) {
	unit := ServiceUnitName(app)
	result, err := RunCommand(ctx, "journalctl", "--user", "-u", unit, "--no-pager", "-n", fmt.Sprintf("%d", lines))
	if err != nil {
		return "", err
	}

	output := strings.TrimSpace(result.Stdout)
	if output == "" {
		output = strings.TrimSpace(result.Stderr)
	}

	if result.ExitCode != 0 {
		return output, fmt.Errorf("journalctl --user -u %s failed: %s", unit, failureDetails(result))
	}

	return output, nil
}

func failureDetails(result CommandResult) string {
	if result.Stderr != "" {
		return result.Stderr
	}
	if result.Stdout != "" {
		return result.Stdout
	}
	if result.ExitCode != 0 {
		return fmt.Sprintf("exit code %d", result.ExitCode)
	}
	return "no output"
}
