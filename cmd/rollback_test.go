package cmd

import (
	"bytes"
	"context"
	"os"
	"testing"

	"deployctl/internal"
)

func TestRollbackCommandDryRunDoesNotMutateFiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	containerDir := t.TempDir()
	app := "demo"

	containerPath := internal.ContainerFilePath(containerDir, app)
	initialContainer := "[Container]\nImage=registry.example.com/demo:current\nPublishPort=8080:80\nContainerName=demo\n"
	if err := os.WriteFile(containerPath, []byte(initialContainer), 0o644); err != nil {
		t.Fatalf("write container file: %v", err)
	}

	statePath, err := internal.StateFilePath(app)
	if err != nil {
		t.Fatalf("StateFilePath: %v", err)
	}
	initialState := internal.StateRecord{
		App:           app,
		PreviousImage: "registry.example.com/demo:previous",
		DeployedImage: "registry.example.com/demo:current",
		DeployedAt:    "2026-03-11T14:22:00Z",
	}
	if err := internal.WriteStateAtomic(statePath, initialState); err != nil {
		t.Fatalf("write state file: %v", err)
	}

	stateBefore, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state file before rollback: %v", err)
	}

	command := newRollbackCmd()
	command.SetContext(withCommandOptions(context.Background(), commandOptions{ContainerDir: containerDir}))
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{app, "--dry-run"})

	if err := command.Execute(); err != nil {
		t.Fatalf("rollback command failed: %v", err)
	}

	const expectedStdout = "rollback=ok\napp=demo\nimage=registry.example.com/demo:previous\ndry_run=true\n"
	if stdout.String() != expectedStdout {
		t.Fatalf("unexpected stdout:\n%s", stdout.String())
	}

	if !bytes.Contains(stderr.Bytes(), []byte("dry-run: would rollback app demo from registry.example.com/demo:current to registry.example.com/demo:previous")) {
		t.Fatalf("unexpected stderr:\n%s", stderr.String())
	}

	containerAfter, err := os.ReadFile(containerPath)
	if err != nil {
		t.Fatalf("read container file after rollback dry-run: %v", err)
	}
	if string(containerAfter) != initialContainer {
		t.Fatalf("container file changed during rollback dry-run:\n%s", string(containerAfter))
	}

	stateAfter, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state file after rollback dry-run: %v", err)
	}
	if !bytes.Equal(stateAfter, stateBefore) {
		t.Fatalf("state file changed during rollback dry-run")
	}
}

func TestLoadOrInitializeRollbackStateUsesExistingLastGood(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	containerDir := t.TempDir()
	app := "demo"
	statePath, err := internal.StateFilePath(app)
	if err != nil {
		t.Fatalf("StateFilePath: %v", err)
	}

	state := internal.StateRecord{
		App:           app,
		PreviousImage: "registry.example.com/demo:prev",
		DeployedImage: "registry.example.com/demo:current",
		DeployedAt:    "2026-03-11T14:22:00Z",
		LastGoodImage: "registry.example.com/demo:last-good",
		LastGoodAt:    "2026-03-11T14:00:00Z",
	}
	if err := internal.WriteStateAtomic(statePath, state); err != nil {
		t.Fatalf("write state file: %v", err)
	}

	got, initialized, stageErr := loadOrInitializeRollbackState(context.Background(), app, containerDir, false)
	if stageErr != nil {
		t.Fatalf("unexpected stage error: %v", stageErr)
	}
	if initialized {
		t.Fatal("expected existing state to avoid initialization")
	}
	if got.RollbackImage() != state.LastGoodImage {
		t.Fatalf("unexpected rollback image: got %q want %q", got.RollbackImage(), state.LastGoodImage)
	}
}
