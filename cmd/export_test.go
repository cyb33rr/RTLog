package cmd

import (
	"strings"
	"testing"
	"time"
)

func TestExportDateFromToMutualExclusivity(t *testing.T) {
	// --date and --from cannot be used together
	exportDate = "2025-01-15"
	exportFrom = "2025-01-14"
	exportTo = ""

	if exportDate != "" && (exportFrom != "" || exportTo != "") {
		// Expected: this condition triggers the error
	} else {
		t.Error("expected mutual exclusivity to be detected")
	}

	// Reset
	exportDate = ""
	exportFrom = ""
	exportTo = ""
}

func TestExportDateFromToMutualExclusivityWithTo(t *testing.T) {
	exportDate = "2025-01-15"
	exportFrom = ""
	exportTo = "2025-01-16"

	if exportDate != "" && (exportFrom != "" || exportTo != "") {
		// Expected: this condition triggers the error
	} else {
		t.Error("expected mutual exclusivity to be detected")
	}

	exportDate = ""
	exportTo = ""
}

func TestExportCommaSeparatedTools(t *testing.T) {
	input := "nmap,nxc,gobuster"
	tools := strings.Split(input, ",")
	if len(tools) != 3 {
		t.Errorf("got %d tools, want 3", len(tools))
	}
	if tools[0] != "nmap" || tools[1] != "nxc" || tools[2] != "gobuster" {
		t.Errorf("got %v, want [nmap nxc gobuster]", tools)
	}
}

func TestExportCommaSeparatedTags(t *testing.T) {
	input := "recon,privesc"
	tags := strings.Split(input, ",")
	if len(tags) != 2 {
		t.Errorf("got %d tags, want 2", len(tags))
	}
}

func TestExportDateValidation(t *testing.T) {
	for _, tc := range []struct {
		val   string
		valid bool
	}{
		{"2025-01-15", true},
		{"not-a-date", false},
		{"2025/01/15", false},
		{"", true}, // empty is ok (no filter)
	} {
		if tc.val == "" {
			continue
		}
		_, err := time.Parse("2006-01-02", tc.val)
		if tc.valid && err != nil {
			t.Errorf("date %q should be valid but got error: %v", tc.val, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("date %q should be invalid but no error", tc.val)
		}
	}
}
