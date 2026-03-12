package internal

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadContainerFileParsesImageAndFields(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "demo.container")
	content := strings.Join([]string{
		"[Unit]",
		"Requires=shared-network-network.service other.service",
		"[Container]",
		"ContainerName=demo",
		"Image=localhost:5000/demo:old # current image",
		"PublishPort=8080:80",
		"PublishPort=8443:443",
		"EnvironmentFile=-%h/.config/demo.env",
		"",
	}, "\n")

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write container fixture: %v", err)
	}

	file, err := LoadContainerFile(path)
	if err != nil {
		t.Fatalf("LoadContainerFile returned error: %v", err)
	}

	if file.Image != "localhost:5000/demo:old" {
		t.Fatalf("unexpected image: %q", file.Image)
	}
	if file.ContainerName != "demo" {
		t.Fatalf("unexpected container name: %q", file.ContainerName)
	}

	if got, want := file.PublishPorts, []string{"8080:80", "8443:443"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected publish ports: got %v want %v", got, want)
	}
	if got, want := file.Requires, []string{"shared-network-network.service", "other.service"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected requires: got %v want %v", got, want)
	}
	if got, want := file.EnvironmentFiles, []string{"-%h/.config/demo.env"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected environment files: got %v want %v", got, want)
	}

	networkUnits := NetworkDependencyUnits(file.Requires)
	if got, want := networkUnits, []string{"shared-network-network.service"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected network dependencies: got %v want %v", got, want)
	}
}

func TestLoadContainerFileRejectsMultipleImageLines(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "bad.container")
	content := strings.Join([]string{
		"[Container]",
		"Image=registry.example.com/demo:one",
		"Image=registry.example.com/demo:two",
		"",
	}, "\n")

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write container fixture: %v", err)
	}

	_, err := LoadContainerFile(path)
	if err == nil {
		t.Fatal("expected error for multiple Image= lines")
	}
	if !strings.Contains(err.Error(), "multiple Image=") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateContainerImageAtomicReplacesOnlyImageLine(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "demo.container")
	initial := strings.Join([]string{
		"[Container]",
		"Image=registry.example.com/demo:old # inline comment",
		"PublishPort=8080:80",
		"",
	}, "\n")

	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatalf("write container fixture: %v", err)
	}

	file, err := LoadContainerFile(path)
	if err != nil {
		t.Fatalf("LoadContainerFile returned error: %v", err)
	}

	if err := UpdateContainerImageAtomic(file, "registry.example.com/demo:new"); err != nil {
		t.Fatalf("UpdateContainerImageAtomic returned error: %v", err)
	}

	updatedBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read updated container file: %v", err)
	}

	expected := strings.Join([]string{
		"[Container]",
		"Image=registry.example.com/demo:new",
		"PublishPort=8080:80",
		"",
	}, "\n")

	if string(updatedBytes) != expected {
		t.Fatalf("unexpected updated container file:\n%s", string(updatedBytes))
	}
}
