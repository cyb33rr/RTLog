package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestState(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	// Create .rt directory.
	os.MkdirAll(filepath.Join(tmpDir, ".rt"), 0755)
	return tmpDir, func() {
		os.Setenv("HOME", origHome)
	}
}

func TestReadStateMissingFile(t *testing.T) {
	_, cleanup := setupTestState(t)
	defer cleanup()

	state := ReadState()
	if state["engagement"] != "" {
		t.Errorf("expected empty engagement, got %q", state["engagement"])
	}
	if state["enabled"] != "1" {
		t.Errorf("expected enabled=1, got %q", state["enabled"])
	}
	if state["capture"] != "1" {
		t.Errorf("expected capture=1, got %q", state["capture"])
	}
}

func TestRoundtrip(t *testing.T) {
	_, cleanup := setupTestState(t)
	defer cleanup()

	original := map[string]string{
		"engagement": "test-op",
		"tag":        "recon",
		"note":       "initial scan",
		"enabled":    "1",
		"capture":    "0",
	}
	if err := WriteState(original); err != nil {
		t.Fatalf("WriteState failed: %v", err)
	}

	got := ReadState()
	for k, want := range original {
		if got[k] != want {
			t.Errorf("key %q: got %q, want %q", k, got[k], want)
		}
	}
}

func TestKeyOrdering(t *testing.T) {
	_, cleanup := setupTestState(t)
	defer cleanup()

	state := map[string]string{
		"capture":    "1",
		"enabled":    "0",
		"engagement": "op1",
		"note":       "hello",
		"tag":        "t1",
	}
	if err := WriteState(state); err != nil {
		t.Fatalf("WriteState failed: %v", err)
	}

	sp := statePath()
	data, err := os.ReadFile(sp)
	if err != nil {
		t.Fatalf("reading state file: %v", err)
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != len(StateKeyOrder) {
		t.Fatalf("expected %d lines, got %d", len(StateKeyOrder), len(lines))
	}
	for i, expected := range StateKeyOrder {
		key := strings.SplitN(lines[i], "=", 2)[0]
		if key != expected {
			t.Errorf("line %d: expected key %q, got %q", i, expected, key)
		}
	}
}

func TestUpdateState(t *testing.T) {
	_, cleanup := setupTestState(t)
	defer cleanup()

	// Write initial state.
	WriteState(map[string]string{
		"engagement": "op1",
		"tag":        "",
		"note":       "",
		"enabled":    "1",
		"capture":    "1",
	})

	got, err := UpdateState(map[string]string{"tag": "privesc", "note": "test note"})
	if err != nil {
		t.Fatalf("UpdateState failed: %v", err)
	}
	if got["tag"] != "privesc" {
		t.Errorf("tag: got %q, want %q", got["tag"], "privesc")
	}
	if got["note"] != "test note" {
		t.Errorf("note: got %q, want %q", got["note"], "test note")
	}
	if got["engagement"] != "op1" {
		t.Errorf("engagement: got %q, want %q", got["engagement"], "op1")
	}
}

func TestNewlineSanitization(t *testing.T) {
	_, cleanup := setupTestState(t)
	defer cleanup()

	state := map[string]string{
		"engagement": "op1",
		"tag":        "has\nnewline",
		"note":       "has\r\nnewline",
		"enabled":    "1",
		"capture":    "1",
	}
	if err := WriteState(state); err != nil {
		t.Fatalf("WriteState failed: %v", err)
	}

	got := ReadState()
	if strings.Contains(got["tag"], "\n") {
		t.Errorf("tag still contains newline: %q", got["tag"])
	}
	if strings.Contains(got["note"], "\r") || strings.Contains(got["note"], "\n") {
		t.Errorf("note still contains newline/cr: %q", got["note"])
	}
}
