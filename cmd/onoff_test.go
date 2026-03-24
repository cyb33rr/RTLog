package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cyb33rr/rtlog/internal/state"
)

func setupOnOffTest(t *testing.T, enabled, capture string) string {
	t.Helper()
	tmpDir := t.TempDir()
	rtDir := filepath.Join(tmpDir, ".rt")
	os.MkdirAll(rtDir, 0755)
	os.WriteFile(filepath.Join(rtDir, "state"),
		[]byte("engagement=test\ntag=\nnote=\nenabled="+enabled+"\ncapture="+capture+"\n"), 0644)
	t.Setenv("HOME", tmpDir)
	return tmpDir
}

func TestOnSetsBothEnabledAndCapture(t *testing.T) {
	setupOnOffTest(t, "0", "0")
	outputFlag = false
	rootCmd.SetArgs([]string{"on"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
	s := state.ReadState()
	if s[state.KeyEnabled] != "1" {
		t.Errorf("enabled = %q, want %q", s[state.KeyEnabled], "1")
	}
	if s[state.KeyCapture] != "1" {
		t.Errorf("capture = %q, want %q", s[state.KeyCapture], "1")
	}
}

func TestOffSetsBothEnabledAndCapture(t *testing.T) {
	setupOnOffTest(t, "1", "1")
	outputFlag = false
	rootCmd.SetArgs([]string{"off"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
	s := state.ReadState()
	if s[state.KeyEnabled] != "0" {
		t.Errorf("enabled = %q, want %q", s[state.KeyEnabled], "0")
	}
	if s[state.KeyCapture] != "0" {
		t.Errorf("capture = %q, want %q", s[state.KeyCapture], "0")
	}
}

func TestOnOutputOnlySetsCapture(t *testing.T) {
	setupOnOffTest(t, "0", "0")
	outputFlag = false
	rootCmd.SetArgs([]string{"on", "--output"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
	s := state.ReadState()
	if s[state.KeyEnabled] != "0" {
		t.Errorf("enabled = %q, want %q (should be untouched)", s[state.KeyEnabled], "0")
	}
	if s[state.KeyCapture] != "1" {
		t.Errorf("capture = %q, want %q", s[state.KeyCapture], "1")
	}
}

func TestOffOutputOnlySetsCapture(t *testing.T) {
	setupOnOffTest(t, "1", "1")
	outputFlag = false
	rootCmd.SetArgs([]string{"off", "--output"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
	s := state.ReadState()
	if s[state.KeyEnabled] != "1" {
		t.Errorf("enabled = %q, want %q (should be untouched)", s[state.KeyEnabled], "1")
	}
	if s[state.KeyCapture] != "0" {
		t.Errorf("capture = %q, want %q", s[state.KeyCapture], "0")
	}
}
