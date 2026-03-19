package display

import (
	"testing"
)

func makeEntries() []Entry {
	return []Entry{
		{"cmd": "nmap -sV 10.0.0.1", "tool": "nmap", "tag": "recon", "note": "scan", "cwd": "/tmp", "exit": 0, "out": ""},
		{"cmd": "gobuster dir -u http://target", "tool": "gobuster", "tag": "recon", "note": "", "cwd": "/opt", "exit": 1, "out": ""},
		{"cmd": "evil-winrm -i 10.0.0.1", "tool": "evil-winrm", "tag": "exploitation", "note": "got shell", "cwd": "/tmp", "exit": 0, "out": "output"},
		{"cmd": "nmap -p- 10.0.0.2", "tool": "nmap", "tag": "recon", "note": "", "cwd": "/tmp", "exit": 0, "out": ""},
		{"cmd": "crackmapexec smb 10.0.0.0/24", "tool": "crackmapexec", "tag": "", "note": "", "cwd": "/tmp", "exit": 2, "out": ""},
	}
}

func TestApplyFiltersNoFilter(t *testing.T) {
	entries := makeEntries()
	filtered := ApplyFilters(entries, "", "", false)
	if len(filtered) != 5 {
		t.Errorf("got %d, want 5 (no filter = all entries)", len(filtered))
	}
}

func TestApplyFiltersTextFilter(t *testing.T) {
	entries := makeEntries()
	filtered := ApplyFilters(entries, "nmap", "", false)
	if len(filtered) != 2 {
		t.Errorf("got %d, want 2 (two nmap entries)", len(filtered))
	}
	if filtered[0] != 0 || filtered[1] != 3 {
		t.Errorf("got indices %v, want [0 3]", filtered)
	}
}

func TestApplyFiltersTextFilterCaseInsensitive(t *testing.T) {
	entries := makeEntries()
	filtered := ApplyFilters(entries, "NMAP", "", false)
	if len(filtered) != 2 {
		t.Errorf("got %d, want 2 (case-insensitive)", len(filtered))
	}
}

func TestApplyFiltersTextFilterMatchesCwd(t *testing.T) {
	entries := makeEntries()
	filtered := ApplyFilters(entries, "/opt", "", false)
	if len(filtered) != 1 {
		t.Errorf("got %d, want 1 (one entry with cwd /opt)", len(filtered))
	}
	if filtered[0] != 1 {
		t.Errorf("got index %d, want 1", filtered[0])
	}
}

func TestApplyFiltersTextFilterMatchesNote(t *testing.T) {
	entries := makeEntries()
	filtered := ApplyFilters(entries, "shell", "", false)
	if len(filtered) != 1 {
		t.Errorf("got %d, want 1 (one entry with 'shell' in note)", len(filtered))
	}
}

func TestApplyFiltersTagFilter(t *testing.T) {
	entries := makeEntries()
	filtered := ApplyFilters(entries, "", "recon", false)
	if len(filtered) != 3 {
		t.Errorf("got %d, want 3 (three recon entries)", len(filtered))
	}
}

func TestApplyFiltersFailOnly(t *testing.T) {
	entries := makeEntries()
	filtered := ApplyFilters(entries, "", "", true)
	if len(filtered) != 2 {
		t.Errorf("got %d, want 2 (two non-zero exit entries)", len(filtered))
	}
}

func TestApplyFiltersStacked(t *testing.T) {
	entries := makeEntries()
	filtered := ApplyFilters(entries, "nmap", "recon", true)
	if len(filtered) != 0 {
		t.Errorf("got %d, want 0 (nmap+recon+fail = none)", len(filtered))
	}
}

func TestApplyFiltersNoMatches(t *testing.T) {
	entries := makeEntries()
	filtered := ApplyFilters(entries, "zzzznotfound", "", false)
	if len(filtered) != 0 {
		t.Errorf("got %d, want 0", len(filtered))
	}
}

func TestCollectTags(t *testing.T) {
	entries := makeEntries()
	tags := CollectTags(entries)

	if len(tags) != 2 {
		t.Fatalf("got %d tags, want 2: %v", len(tags), tags)
	}
	// CollectTags returns sorted tags
	if tags[0] != "exploitation" || tags[1] != "recon" {
		t.Errorf("got tags %v, want [exploitation recon]", tags)
	}
}

func TestCollectTagsNoTags(t *testing.T) {
	entries := []Entry{
		{"cmd": "ls", "tool": "ls", "tag": "", "exit": 0},
	}
	tags := CollectTags(entries)
	if len(tags) != 0 {
		t.Errorf("got %d tags, want 0 for entries with no tags", len(tags))
	}
}
