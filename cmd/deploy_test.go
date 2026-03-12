package cmd

import (
	"context"
	"errors"
	"os"
	"testing"

	"deployctl/internal"
)

func TestRunDeployWorkflowDryRunDoesNotMutateFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	containerDir := t.TempDir()
	app := "demo"
	containerPath := internal.ContainerFilePath(containerDir, app)
	initial := "[Container]\nImage=registry.example.com/demo:old\nPublishPort=8080:80\nContainerName=demo\n"

	if err := os.WriteFile(containerPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write container file: %v", err)
	}

	request := deployRequest{
		App:          app,
		Image:        "registry.example.com/demo:new",
		ContainerDir: containerDir,
		DryRun:       true,
	}

	result, stageErr := runDeployWorkflow(context.Background(), request)
	if stageErr != nil {
		t.Fatalf("runDeployWorkflow returned stage error: %v", stageErr)
	}

	if !result.DryRun {
		t.Fatal("expected DryRun result to be true")
	}
	if result.Noop {
		t.Fatal("expected dry-run deploy to not be noop when image changes")
	}
	if result.PreviousImage != "registry.example.com/demo:old" {
		t.Fatalf("unexpected previous image: %q", result.PreviousImage)
	}

	updated, err := os.ReadFile(containerPath)
	if err != nil {
		t.Fatalf("read container file after dry-run: %v", err)
	}
	if string(updated) != initial {
		t.Fatalf("container file changed during dry-run:\n%s", string(updated))
	}

	statePath, err := internal.StateFilePath(app)
	if err != nil {
		t.Fatalf("StateFilePath: %v", err)
	}
	_, err = os.Stat(statePath)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no state file on dry-run, got err=%v", err)
	}
}
