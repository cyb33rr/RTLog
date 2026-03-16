package state

import (
	"os"
	"path/filepath"
	"strings"
)

// State key constants.
const (
	KeyEngagement = "engagement"
	KeyTag        = "tag"
	KeyNote       = "note"
	KeyEnabled    = "enabled"
	KeyCapture    = "capture"
)

// StateKeyOrder defines the canonical ordering of keys in the state file.
var StateKeyOrder = []string{KeyEngagement, KeyTag, KeyNote, KeyEnabled, KeyCapture}

// StateDefaults provides default values for each state key.
var StateDefaults = map[string]string{
	KeyEngagement: "",
	KeyTag:        "",
	KeyNote:       "",
	KeyEnabled:    "1",
	KeyCapture:    "1",
}

// statePath returns the path to the state file (~/.rt/state).
func statePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".rt", "state")
	}
	return filepath.Join(home, ".rt", "state")
}

// ReadState parses ~/.rt/state and returns a map with defaults for missing keys.
func ReadState() map[string]string {
	state := make(map[string]string, len(StateDefaults))
	for k, v := range StateDefaults {
		state[k] = v
	}

	data, err := os.ReadFile(statePath())
	if err != nil {
		return state
	}

	for _, line := range strings.Split(string(data), "\n") {
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := line[:idx]
		val := line[idx+1:]
		if _, ok := StateDefaults[key]; ok {
			state[key] = val
		}
	}
	return state
}

// WriteState writes the state map atomically to ~/.rt/state.
// Keys are written in canonical order. Newlines in values are sanitized.
func WriteState(state map[string]string) error {
	sp := statePath()
	dir := filepath.Dir(sp)

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".state.")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	defer func() {
		// Clean up temp file on failure.
		_ = os.Remove(tmpName)
	}()

	for _, key := range StateKeyOrder {
		val, ok := state[key]
		if !ok {
			val = StateDefaults[key]
		}
		// Sanitize newlines in values.
		val = strings.ReplaceAll(val, "\n", " ")
		val = strings.ReplaceAll(val, "\r", "")
		if _, err := tmp.WriteString(key + "=" + val + "\n"); err != nil {
			tmp.Close()
			return err
		}
	}

	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, sp)
}

// UpdateState reads the current state, merges the provided key-value pairs,
// and writes it back atomically.
func UpdateState(kwargs map[string]string) (map[string]string, error) {
	state := ReadState()
	for k, v := range kwargs {
		state[k] = v
	}
	if err := WriteState(state); err != nil {
		return nil, err
	}
	return state, nil
}
