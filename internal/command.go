package internal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

func (r CommandResult) CombinedOutput() string {
	stdout := strings.TrimSpace(r.Stdout)
	stderr := strings.TrimSpace(r.Stderr)

	if stdout != "" && stderr != "" {
		return stdout + "\n" + stderr
	}
	if stderr != "" {
		return stderr
	}
	return stdout
}

func RunCommand(ctx context.Context, command string, args ...string) (CommandResult, error) {
	cmd := exec.CommandContext(ctx, command, args...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := CommandResult{
		Stdout: strings.TrimSpace(stdout.String()),
		Stderr: strings.TrimSpace(stderr.String()),
	}

	if err == nil {
		result.ExitCode = 0
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}

	return result, fmt.Errorf("run %s: %w", command, err)
}
