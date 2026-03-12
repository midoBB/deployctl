package internal

import (
	"path/filepath"
	"testing"
)

func TestWriteStateAtomicAndReadStateRoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), ".deployctl", "demo.state.json")
	input := StateRecord{
		App:           "demo",
		PreviousImage: "registry.example.com/demo:old",
		DeployedImage: "registry.example.com/demo:new",
		DeployedAt:    "2026-03-11T14:22:00Z",
	}

	if err := WriteStateAtomic(path, input); err != nil {
		t.Fatalf("WriteStateAtomic returned error: %v", err)
	}

	got, err := ReadState(path)
	if err != nil {
		t.Fatalf("ReadState returned error: %v", err)
	}

	if *got != input {
		t.Fatalf("round-trip mismatch: got %#v want %#v", *got, input)
	}
}

func TestStateRecordRollbackImagePrefersLastGood(t *testing.T) {
	t.Parallel()

	state := StateRecord{
		PreviousImage: "registry.example.com/demo:prev",
		DeployedImage: "registry.example.com/demo:deployed",
		LastGoodImage: "registry.example.com/demo:last-good",
	}

	if got, want := state.RollbackImage(), "registry.example.com/demo:last-good"; got != want {
		t.Fatalf("unexpected rollback image: got %q want %q", got, want)
	}
}

func TestStateRecordRollbackImageFallbackOrder(t *testing.T) {
	t.Parallel()

	state := StateRecord{
		PreviousImage: "registry.example.com/demo:prev",
		DeployedImage: "registry.example.com/demo:deployed",
	}

	if got, want := state.RollbackImage(), "registry.example.com/demo:prev"; got != want {
		t.Fatalf("unexpected rollback image with previous fallback: got %q want %q", got, want)
	}

	state.PreviousImage = ""
	if got, want := state.RollbackImage(), "registry.example.com/demo:deployed"; got != want {
		t.Fatalf("unexpected rollback image with deployed fallback: got %q want %q", got, want)
	}
}
