package internal

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestPrintOutputKeyValue(t *testing.T) {
	t.Parallel()

	fields := []Field{
		{Key: "deploy", Value: "ok"},
		{Key: "app", Value: "demo"},
		{Key: "dry_run", Value: true},
	}

	var out bytes.Buffer
	if err := PrintOutput(&out, false, fields); err != nil {
		t.Fatalf("PrintOutput returned error: %v", err)
	}

	const expected = "deploy=ok\napp=demo\ndry_run=true\n"
	if out.String() != expected {
		t.Fatalf("unexpected key=value output:\n%s", out.String())
	}
}

func TestPrintOutputJSON(t *testing.T) {
	t.Parallel()

	fields := []Field{
		{Key: "deploy", Value: "ok"},
		{Key: "app", Value: "demo"},
		{Key: "attempts", Value: 2},
		{Key: "empty", Value: nil},
	}

	var out bytes.Buffer
	if err := PrintOutput(&out, true, fields); err != nil {
		t.Fatalf("PrintOutput returned error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if got := payload["deploy"]; got != "ok" {
		t.Fatalf("unexpected deploy value: %v", got)
	}
	if got := payload["app"]; got != "demo" {
		t.Fatalf("unexpected app value: %v", got)
	}
	if got := payload["attempts"]; got != float64(2) {
		t.Fatalf("unexpected attempts value: %v", got)
	}
	if got := payload["empty"]; got != "" {
		t.Fatalf("unexpected empty value: %v", got)
	}
}
