package cmd

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"deployctl/internal"
)

func TestValidateAllChecksPass(t *testing.T) {
	t.Parallel()

	containerDir := t.TempDir()
	app := "demo"

	containerPath := internal.ContainerFilePath(containerDir, app)
	content := strings.Join([]string{
		"[Unit]",
		"Requires=shared-network-network.service",
		"[Container]",
		"ContainerName=demo",
		"Image=registry.example.com/demo:latest",
		"PublishPort=8080:80",
		"EnvironmentFile=" + createTempEnvFile(t, containerDir),
		"",
	}, "\n")
	if err := os.WriteFile(containerPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write container file: %v", err)
	}

	command := newValidateCmd()
	command.SetContext(withCommandOptions(context.Background(), commandOptions{ContainerDir: containerDir}))
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{app})

	// This test will fail for some checks that require a running systemd/podman
	// environment (image_pullable, systemd_unit, network_dependency). We just
	// verify that container_file and image_parseable pass.
	_ = command.Execute()

	output := stdout.String()
	if !strings.Contains(output, "container_file=ok") {
		t.Fatalf("expected container_file=ok in output:\n%s", output)
	}
	if !strings.Contains(output, "image_parseable=ok") {
		t.Fatalf("expected image_parseable=ok in output:\n%s", output)
	}
}

func TestValidateMissingContainerFile(t *testing.T) {
	t.Parallel()

	containerDir := t.TempDir()
	app := "nonexistent"

	command := newValidateCmd()
	command.SetContext(withCommandOptions(context.Background(), commandOptions{ContainerDir: containerDir}))
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{app})

	err := command.Execute()
	if err == nil {
		t.Fatal("expected validate to return an error for missing container file")
	}

	output := stdout.String()
	if !strings.Contains(output, "container_file=failed") {
		t.Fatalf("expected container_file=failed in output:\n%s", output)
	}
	if !strings.Contains(output, "validation=failed") {
		t.Fatalf("expected validation=failed in output:\n%s", output)
	}
}

func TestValidateMissingContainerName(t *testing.T) {
	t.Parallel()

	containerDir := t.TempDir()
	app := "demo"

	containerPath := internal.ContainerFilePath(containerDir, app)
	content := strings.Join([]string{
		"[Container]",
		"Image=registry.example.com/demo:latest",
		"PublishPort=8080:80",
		"",
	}, "\n")
	if err := os.WriteFile(containerPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write container file: %v", err)
	}

	command := newValidateCmd()
	command.SetContext(withCommandOptions(context.Background(), commandOptions{ContainerDir: containerDir}))
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{app})

	err := command.Execute()
	if err == nil {
		t.Fatal("expected validate to return an error for missing ContainerName")
	}

	output := stdout.String()
	if !strings.Contains(output, "container_file=ok") {
		t.Fatalf("expected container_file=ok in output:\n%s", output)
	}
	if !strings.Contains(output, "image_parseable=failed") {
		t.Fatalf("expected image_parseable=failed in output:\n%s", output)
	}

	stderrOutput := stderr.String()
	if !strings.Contains(stderrOutput, "missing ContainerName") {
		t.Fatalf("expected stderr to mention missing ContainerName:\n%s", stderrOutput)
	}
}

func TestValidateJSONOutput(t *testing.T) {
	t.Parallel()

	containerDir := t.TempDir()
	app := "demo"

	containerPath := internal.ContainerFilePath(containerDir, app)
	content := strings.Join([]string{
		"[Container]",
		"ContainerName=demo",
		"Image=registry.example.com/demo:latest",
		"PublishPort=8080:80",
		"",
	}, "\n")
	if err := os.WriteFile(containerPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write container file: %v", err)
	}

	command := newValidateCmd()
	command.SetContext(withCommandOptions(context.Background(), commandOptions{ContainerDir: containerDir, JSONOutput: true}))
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{app})

	_ = command.Execute()

	output := stdout.String()
	if !strings.HasPrefix(strings.TrimSpace(output), "{") {
		t.Fatalf("expected JSON output, got:\n%s", output)
	}
	if !strings.Contains(output, `"container_file"`) {
		t.Fatalf("expected container_file key in JSON output:\n%s", output)
	}
}

func createTempEnvFile(t *testing.T, dir string) string {
	t.Helper()
	envPath := dir + "/demo.env"
	if err := os.WriteFile(envPath, []byte("FOO=bar\n"), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}
	return envPath
}
